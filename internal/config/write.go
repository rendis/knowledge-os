package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"go.yaml.in/yaml/v3"
)

// Edit serializes cooperative writers, checks optional expected file hash and
// preserves YAML comments and unknown fields. It never follows a target symlink.
func Edit(path, expected string, create bool, validate func(Object) error, change func(*yaml.Node) error) error {
	parent, e := CanonicalRoot(filepath.Dir(path))
	if e != nil {
		return e
	}
	path = filepath.Join(parent, filepath.Base(path))
	if st, e := os.Lstat(path + ".lock"); e == nil && !st.Mode().IsRegular() {
		return errors.New("configuration lock must be regular")
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	lock := flock.New(path + ".lock")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ok, e := lock.TryLockContext(ctx, 20*time.Millisecond)
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("configuration is locked")
	}
	defer lock.Close()
	n, m, original, e := readYAML(path)
	mode := os.FileMode(0600)
	if os.IsNotExist(e) && create {
		original = nil
		n = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
		m = Object{}
		e = nil
	} else if e == nil {
		st, se := os.Stat(path)
		if se != nil {
			return se
		}
		mode = st.Mode().Perm()
		if e = validate(m); e != nil {
			return e
		}
	}
	if e != nil {
		return e
	}
	sum := sha256.Sum256(original)
	if expected != "" && expected != hex.EncodeToString(sum[:]) {
		return errors.New("configuration changed: expected hash does not match")
	}
	if e = change(n.Content[0]); e != nil {
		return e
	}
	m = Object{}
	if e = n.Decode(&m); e != nil {
		return e
	}
	if e = validate(m); e != nil {
		return e
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if e = enc.Encode(n); e != nil {
		return e
	}
	if e = enc.Close(); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".vaultctl-config-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(mode); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(buf.Bytes()); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	// Recheck non-cooperative edits immediately before atomic replacement.
	current, e := readConfigBytes(path)
	if os.IsNotExist(e) && original == nil {
		e = nil
	}
	if e != nil {
		return e
	}
	if !bytes.Equal(current, original) {
		return errors.New("configuration changed during update")
	}
	if st, e := os.Lstat(path); e == nil && !st.Mode().IsRegular() {
		return errors.New("configuration target is not regular")
	}
	if e = os.Rename(tmp, path); e != nil {
		return fmt.Errorf("replace config: %w", e)
	}
	return nil
}
func nodeMap(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			v := n.Content[i+1]
			if v.Tag == "!!null" {
				v.Kind = yaml.MappingNode
				v.Tag = "!!map"
				v.Value = ""
			}
			return v
		}
	}
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
	return v
}
func nodeSet(n *yaml.Node, key string, value any) error {
	var v yaml.Node
	if e := v.Encode(value); e != nil {
		return e
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			old := n.Content[i+1]
			v.HeadComment = old.HeadComment
			v.LineComment = old.LineComment
			v.FootComment = old.FootComment
			n.Content[i+1] = &v
			return nil
		}
	}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &v)
	return nil
}
func Bind(root, capability string, procedures []string, expected string) (Object, error) {
	if !kebab.MatchString(capability) || len(procedures) == 0 {
		return nil, errors.New("bind requires capability and procedures")
	}
	e := Edit(filepath.Join(root, "instance.yaml"), expected, false, ValidateInstance, func(n *yaml.Node) error {
		if e := nodeSet(nodeMap(n, "capabilities"), capability, procedures); e != nil {
			return e
		}
		var m Object
		if e := n.Decode(&m); e != nil {
			return e
		}
		if e := ValidateInstance(m); e != nil {
			return e
		}
		_, e := ResolveCapability(root, m, capability)
		return e
	})
	if e != nil {
		return nil, e
	}
	m, e := LoadInstance(root)
	if e != nil {
		return nil, e
	}
	return ResolveCapability(root, m, capability)
}

type WorkspaceUpdate struct {
	Roots        []string
	ManagedRoot  *string
	WorktreeRoot *string
	Ports        map[string]string
	Initialize   bool
	ExpectedHash string
}

func UpdateWorkspace(root string, u WorkspaceUpdate) (Object, error) {
	e := Edit(filepath.Join(root, WorkspaceFile), u.ExpectedHash, true, ValidateWorkspace, func(n *yaml.Node) error {
		if e := nodeSet(n, "version", 1); e != nil {
			return e
		}
		w := nodeMap(n, "workspace")
		skills := nodeMap(n, "skills")
		if u.Roots != nil || u.Initialize {
			roots := u.Roots
			if roots == nil {
				roots = []string{}
			}
			if e := nodeSet(w, "repository_roots", roots); e != nil {
				return e
			}
		}
		if u.ManagedRoot != nil || u.Initialize {
			s := ""
			if u.ManagedRoot != nil {
				s = *u.ManagedRoot
			}
			managed := nodeMap(w, "managed_clone")
			if e := nodeSet(managed, "enabled", s != ""); e != nil {
				return e
			}
			if e := nodeSet(managed, "root", s); e != nil {
				return e
			}
		}
		if u.WorktreeRoot != nil || u.Initialize {
			s := ""
			if u.WorktreeRoot != nil {
				s = *u.WorktreeRoot
			}
			if e := nodeSet(nodeMap(skills, "manage-development-handoff"), "worktree_root", s); e != nil {
				return e
			}
		}
		env := nodeMap(nodeMap(skills, "inspect-database"), "environments")
		for name, port := range u.Ports {
			p, e := portValue(port)
			if e != nil {
				return e
			}
			if old := findNode(env, name); old != nil && old.Kind == yaml.MappingNode {
				if e = nodeSet(old, "proxy_port", p); e != nil {
					return e
				}
			} else {
				if e = nodeSet(env, name, map[string]string{"proxy_port": p}); e != nil {
					return e
				}
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return Workspace(root)
}
func findNode(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
