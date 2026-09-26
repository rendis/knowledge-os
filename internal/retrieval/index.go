// Package retrieval maintains a disposable, local Markdown search index.
package retrieval

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = "2"
const maxFileBytes = 16 << 20

type Options struct {
	Vault, Cache string
	Rebuild      bool
}
type Index struct {
	db    *sql.DB
	Root  string
	Stats Refresh
}
type Refresh struct {
	Files        int   `json:"files"`
	Updated      int   `json:"updated"`
	Deleted      int   `json:"deleted"`
	Unchanged    int   `json:"unchanged"`
	Milliseconds int64 `json:"milliseconds"`
}

func canonicalRoot(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	for _, name := range []string{"instance.yaml", "00-Home.md"} {
		st, err := os.Lstat(filepath.Join(root, name))
		if err != nil {
			return "", fmt.Errorf("vault requires %s: %w", name, err)
		}
		if !st.Mode().IsRegular() {
			return "", fmt.Errorf("vault marker %s must be a regular file", name)
		}
	}
	return root, nil
}

// Eligibility is independent of Git and Obsidian exclusions: private knowledge
// may be searchable locally, while control files and arbitrary scratch are not.
func eligible(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) == 1 {
		return parts[0] == "00-Home.md"
	}
	switch parts[0] {
	case "10-Sistemas", "15-Arquitectura", "20-Repos", "25-Topics", "30-Flujos", "40-Integraciones", "50-Glosario", "60-Operacion", "70-Aprendizajes", "90-Meta", "investigations", ".investigations", ".investigations-private":
	default:
		return false
	}
	for _, p := range parts[1 : len(parts)-1] {
		if strings.HasPrefix(p, ".") {
			return false
		}
	}
	return strings.EqualFold(filepath.Ext(path), ".md") && !strings.HasPrefix(parts[len(parts)-1], ".")
}

// Resolve existing ancestors even when the cache itself has not been created.
func canonicalFuturePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return real, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", err
	}
	real, err = canonicalFuturePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(real, filepath.Base(abs)), nil
}

func Open(ctx context.Context, opt Options) (*Index, error) {
	start := time.Now()
	root, err := canonicalRoot(opt.Vault)
	if err != nil {
		return nil, err
	}
	cache := opt.Cache
	if cache == "" {
		cache, err = os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		cache = filepath.Join(cache, "kos")
	}
	cache, err = canonicalFuturePath(cache)
	if err != nil {
		return nil, err
	}
	// Indexes may contain private text; never put them under a versioned vault.
	if rel, e := filepath.Rel(root, cache); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("cache must be outside the vault")
	}
	if err = os.MkdirAll(cache, 0700); err != nil {
		return nil, err
	}
	st, err := os.Lstat(cache)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("cache must be a real directory")
	}
	sum := sha256.Sum256([]byte(root))
	file := filepath.Join(cache, hex.EncodeToString(sum[:])+".sqlite")
	if st, e := os.Lstat(file); e == nil && !st.Mode().IsRegular() {
		return nil, errors.New("index must be a regular file")
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	uri := sqliteFileURL(filepath.ToSlash(file))
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "journal_mode(DELETE)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Set("_txlock", "immediate")
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	idx := &Index{db: db, Root: root}
	if err = idx.refresh(ctx, opt.Rebuild); err != nil {
		db.Close()
		return nil, err
	}
	idx.Stats.Milliseconds = time.Since(start).Milliseconds()
	return idx, nil
}
func (i *Index) Close() error { return i.db.Close() }

// A Windows drive is a path segment, never a URI authority. A leading slash
// produces file:///C:/... rather than file://C:/..., which SQLite rejects.
func sqliteFileURL(path string) url.URL {
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	return url.URL{Scheme: "file", Path: path}
}

func (i *Index) refresh(ctx context.Context, rebuild bool) error {
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY,value TEXT NOT NULL)`); err != nil {
		return err
	}
	var version string
	err = tx.QueryRowContext(ctx, `SELECT value FROM meta WHERE key='schema'`).Scan(&version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if version != "" && version != schemaVersion && !rebuild {
		return fmt.Errorf("index schema %s unsupported: use index --rebuild", version)
	}
	if rebuild {
		if _, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS passages; DROP TABLE IF EXISTS files; DROP TABLE IF EXISTS edges`); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS files(path TEXT PRIMARY KEY, hash TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS edges(path TEXT NOT NULL,field TEXT NOT NULL,target TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS edges_path ON edges(path);
 CREATE VIRTUAL TABLE IF NOT EXISTS passages USING fts5(path UNINDEXED,title,aliases,section,body,line UNINDEXED,origin UNINDEXED,visibility UNINDEXED,hash UNINDEXED,tokenize='unicode61 remove_diacritics 2');`)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO meta VALUES('schema',?)`, schemaVersion); err != nil {
		return err
	}
	old := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT path,hash FROM files`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p, h string
		if err = rows.Scan(&p, &h); err != nil {
			rows.Close()
			return err
		}
		old[p] = h
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	insert, err := tx.PrepareContext(ctx, `INSERT INTO passages(path,title,aliases,section,body,line,origin,visibility,hash) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	vault, err := os.OpenRoot(i.Root)
	if err != nil {
		return err
	}
	defer vault.Close()
	err = filepath.WalkDir(i.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(i.Root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel != "." && !strings.Contains(rel, string(filepath.Separator)) && !eligible(filepath.Join(rel, "placeholder.md")) {
				return filepath.SkipDir
			}
			if rel != "." && strings.Contains(rel, string(filepath.Separator)) && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !eligible(rel) {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("eligible note is a symlink: %s", rel)
		}
		before, err := vault.Stat(rel)
		if err != nil {
			return err
		}
		if !before.Mode().IsRegular() {
			return fmt.Errorf("not a regular note: %s", rel)
		}
		if before.Size() > maxFileBytes {
			return fmt.Errorf("note exceeds %d bytes: %s", maxFileBytes, rel)
		}
		// Bound each read even if a concurrent writer grows the file after Stat.
		handle, err := vault.Open(rel)
		if err != nil {
			return err
		}
		data, err := readBounded(handle, maxFileBytes)
		closeErr := handle.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if closeErr != nil {
			return closeErr
		}
		after, err := vault.Stat(rel)
		if err != nil {
			return err
		}
		if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return fmt.Errorf("note changed while reading: %s; retry", rel)
		}
		hash := sha256.Sum256(data)
		digest := hex.EncodeToString(hash[:])
		p := filepath.ToSlash(rel)
		i.Stats.Files++
		if old[p] == digest {
			i.Stats.Unchanged++
			delete(old, p)
			return nil
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM passages WHERE path=?`, p); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM edges WHERE path=?`, p); err != nil {
			return err
		}
		doc, err := parseDocument(p, data)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		for _, edge := range doc.Edges {
			if _, err = tx.ExecContext(ctx, `INSERT INTO edges VALUES(?,?,?)`, p, edge.Field, edge.Target); err != nil {
				return err
			}
		}
		for _, chunk := range doc.Passages {
			if _, err = insert.ExecContext(ctx, p, doc.Title, doc.Aliases, chunk.Section, chunk.Body, chunk.Line, doc.Origin, doc.Visibility, digest); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO files VALUES(?,?)`, p, digest); err != nil {
			return err
		}
		delete(old, p)
		i.Stats.Updated++
		return nil
	})
	if err != nil {
		return err
	}
	for p := range old {
		if _, err = tx.ExecContext(ctx, `DELETE FROM edges WHERE path=?`, p); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM passages WHERE path=?`, p); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM files WHERE path=?`, p); err != nil {
			return err
		}
		i.Stats.Deleted++
	}
	return tx.Commit()
}
