// Package release finds, verifies and installs kos releases, and tells the agent when one is newer.
package release

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub repository that publishes kos releases (KOS_REPO overrides it). Private
// repositories are read through the GitHub CLI and the developer's login.
var Repo = "rendis/knowledge-os"

func repo() string {
	if r := os.Getenv("KOS_REPO"); r != "" {
		return r
	}
	return Repo
}

// Asset is the release file for this platform.
func Asset() string {
	name := "kos-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Compare orders two versions like 0.15.2 (a missing or non-numeric part counts as 0).
func Compare(a, b string) int {
	pa, pb := strings.Split(strings.TrimPrefix(a, "v"), "."), strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func ghToken() []string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return []string{"GH_TOKEN=" + t}
	}
	return nil
}

func gh(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Env = append(os.Environ(), ghToken()...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		return nil, fmt.Errorf("gh %s: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return b, nil
}

func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return nil, e
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" && strings.Contains(url, "github.com") {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

// Latest returns the newest published version. KOS_DOWNLOAD_URL serves a directory with a VERSION
// file and the assets (mirrors, tests); otherwise the GitHub CLI reads the repository, which works for
// private ones, and plain HTTPS is the fallback for public ones.
func Latest(ctx context.Context) (string, error) {
	if base := os.Getenv("KOS_DOWNLOAD_URL"); base != "" {
		b, e := httpGet(ctx, strings.TrimRight(base, "/")+"/VERSION")
		return strings.TrimSpace(string(b)), e
	}
	if _, e := exec.LookPath("gh"); e == nil {
		b, e := gh(ctx, "api", "repos/"+repo()+"/releases/latest", "--jq", ".tag_name")
		if e == nil {
			return strings.TrimPrefix(strings.TrimSpace(string(b)), "v"), nil
		}
	}
	b, e := httpGet(ctx, "https://api.github.com/repos/"+repo()+"/releases/latest")
	if e != nil {
		return "", fmt.Errorf("%v (a private repository needs the GitHub CLI logged in with access to %s)", e, repo())
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if e := json.Unmarshal(b, &rel); e != nil || rel.Tag == "" {
		return "", errors.New("unreadable latest release")
	}
	return strings.TrimPrefix(rel.Tag, "v"), nil
}

// fetch downloads this platform's asset and SHA256SUMS of a version into dir.
func fetch(ctx context.Context, version, dir string) error {
	asset := Asset()
	if base := os.Getenv("KOS_DOWNLOAD_URL"); base != "" {
		for _, f := range []string{asset, "SHA256SUMS"} {
			b, e := httpGet(ctx, strings.TrimRight(base, "/")+"/"+f)
			if e != nil {
				return e
			}
			if e := os.WriteFile(filepath.Join(dir, f), b, 0o600); e != nil {
				return e
			}
		}
		return nil
	}
	if _, e := exec.LookPath("gh"); e == nil {
		if _, e := gh(ctx, "release", "download", "v"+version, "--repo", repo(), "--pattern", asset, "--pattern", "SHA256SUMS", "--dir", dir, "--clobber"); e == nil {
			return nil
		}
	}
	for _, f := range []string{asset, "SHA256SUMS"} {
		b, e := httpGet(ctx, "https://github.com/"+repo()+"/releases/download/v"+version+"/"+f)
		if e != nil {
			return e
		}
		if e := os.WriteFile(filepath.Join(dir, f), b, 0o600); e != nil {
			return e
		}
	}
	return nil
}

func verify(dir string) error {
	f, e := os.Open(filepath.Join(dir, "SHA256SUMS"))
	if e != nil {
		return e
	}
	defer f.Close()
	want := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		if parts := strings.Fields(s.Text()); len(parts) == 2 && strings.TrimPrefix(parts[1], "*") == Asset() {
			want = parts[0]
		}
	}
	if e := s.Err(); e != nil {
		return e
	}
	if want == "" {
		return fmt.Errorf("SHA256SUMS lists no %s", Asset())
	}
	b, e := os.ReadFile(filepath.Join(dir, Asset()))
	if e != nil {
		return e
	}
	got := sha256.Sum256(b)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s: the download is refused", Asset())
	}
	return nil
}

// Update replaces the kos at self with the given version (the latest when empty).
func Update(ctx context.Context, current, to, self string) (map[string]any, error) {
	if to == "" {
		v, e := Latest(ctx)
		if e != nil {
			return nil, e
		}
		to = v
	}
	if Compare(to, current) == 0 {
		return map[string]any{"status": "current", "version": current}, nil
	}
	dir, e := os.MkdirTemp("", "kos-update-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(dir)
	if e := fetch(ctx, to, dir); e != nil {
		return nil, e
	}
	if e := verify(dir); e != nil {
		return nil, e
	}
	if real, e := filepath.EvalSymlinks(self); e == nil {
		self = real
	}
	b, _ := os.ReadFile(filepath.Join(dir, Asset()))
	tmp := self + ".new"
	if e := os.WriteFile(tmp, b, 0o755); e != nil {
		return nil, fmt.Errorf("cannot write next to %s: %w", self, e)
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(self + ".old")
		if e := os.Rename(self, self+".old"); e != nil {
			return nil, e
		}
	}
	if e := os.Rename(tmp, self); e != nil {
		return nil, e
	}
	_ = saveCache(cacheEntry{CheckedAt: time.Now(), Latest: to})
	return map[string]any{"status": "updated", "from": current, "to": to, "path": self,
		"next": "run `kos kernel status` in each vault: a newer kos may carry a newer kernel"}, nil
}

type cacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

func cachePath() string {
	dir, e := os.UserCacheDir()
	if e != nil {
		return ""
	}
	return filepath.Join(dir, "knowledge-os", "latest.json")
}

func saveCache(c cacheEntry) error {
	p := cachePath()
	if p == "" {
		return errors.New("no cache directory")
	}
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	b, _ := json.Marshal(c)
	return os.WriteFile(p, b, 0o644)
}

// Newer returns the latest version when it is newer than current. It asks the release source at most
// once a day with a short timeout, never in CI or with KOS_NO_UPDATE_CHECK set, and never fails a command.
func Newer(current string) string {
	if current == "dev" || os.Getenv("CI") != "" || os.Getenv("KOS_NO_UPDATE_CHECK") != "" {
		return ""
	}
	var c cacheEntry
	if b, e := os.ReadFile(cachePath()); e == nil {
		_ = json.Unmarshal(b, &c)
	}
	if time.Since(c.CheckedAt) > 24*time.Hour {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if v, e := Latest(ctx); e == nil && v != "" {
			c = cacheEntry{CheckedAt: time.Now(), Latest: v}
		} else {
			c.CheckedAt = time.Now() // an unreachable source is retried tomorrow, not on every command
		}
		_ = saveCache(c)
	}
	if c.Latest != "" && Compare(c.Latest, current) > 0 {
		return c.Latest
	}
	return ""
}
