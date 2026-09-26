package discover

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Configuration entries come from every non-code text file. The format is detected from
// the file content and extension family; no file name or framework list decides meaning.

type entry struct {
	File         string   `json:"file"`
	KeyPath      string   `json:"key_path"`
	Key          string   `json:"key"`
	Value        string   `json:"value"`
	Context      string   `json:"context,omitempty"`
	Secret       bool     `json:"secret,omitempty"`
	Credential   bool     `json:"credential,omitempty"` // a literal credential versioned in the source
	Placeholders []string `json:"placeholders,omitempty"`
}

var nonConfigExt = map[string]bool{}

func init() {
	for _, e := range strings.Fields(".png .jpg .jpeg .gif .svg .ico .webp .bmp .ttf .otf .woff .woff2 .eot .pdf .jar .war .zip .gz .tgz .so .dll .exe .bin .pem .crt .der .p12 .jks .keystore .lock .sum .md .markdown .txt .rst .html .htm .css .scss .less .map .lst .h .c .cc .cpp .s .proto .csv .xlsx .mp4 .mp3 .ipynb") {
		nonConfigExt[e] = true
	}
	for e := range codeExt {
		nonConfigExt[e] = true
	}
}

var (
	testConfig = regexp.MustCompile(`(?i)(^|/)(tests?|__tests__|mocks?|fixtures?|testdata|e2e)/|\.(test|spec|e2e)\.|(^|/)\.env\.test`)
	// Files that describe, template or tool around a deployment instead of being one: agent tooling and
	// developer tools kept in the repository, documentation, examples and backups.
	nonRuntime = regexp.MustCompile(`(?i)(^|/)\.(agents|claude|codex|cursor|windsurf)/|(^|/)(docs?|examples?|tools)/|\.(example|sample|template|tpl|bkp|bak|orig|old)$|(^|/)[^/]*\.(example|sample)\.[^/]+$|(^|/)example[^/]*\.(ya?ml|json|env)$`)
	secretFile = regexp.MustCompile(`(?i)secret|credential|\.pem$|\.key$`)
	// A credential inside an ordinary configuration file: never judged, stored or sent anywhere.
	credentialKey   = regexp.MustCompile(`(?i)(^|[_.\-])(pass(word|wd)?|pwd|secret|token|api[_\-]?key|apikey|private[_\-]?key|client[_\-]?secret|access[_\-]?key|account[_\-]?key|auth[_\-]?key|sas|signature|credentials?)($|[_.\-])`)
	credentialValue = regexp.MustCompile(`(?i)[?&](sig|signature|x-amz-signature|x-amz-credential|x-goog-signature|x-goog-credential|access_token|token|api[_\-]?key|key|code|password|pwd)=[^&\s]{6,}|(password|pwd|sharedaccesskey|accountkey|accesskey)\s*=\s*[^;\s]{4,}|://[^/\s:@]+:[^/\s@]{3,}@|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_\-]{35}|gh[pousr]_[0-9A-Za-z]{30,}|xox[abprs]-[0-9A-Za-z\-]{10,}|-----BEGIN [A-Z ]*PRIVATE KEY`)
	secretPointer   = regexp.MustCompile(`(?i)(^|[_.\-])(name|id|ref|reference|path|file|arn|url|uri|endpoint|version|header|type|ttl|expiry|expiration|length|enabled)($|[_.\-])`)
	// A placeholder a developer is expected to replace is not a credential.
	exampleValue = regexp.MustCompile(`(?i)^(postgres|password|passw0rd|admin|root|secret|changeme|change[_\-]?me|example|dummy|sample|test|testing|x{3,}|\*{3,}|your[_\-].*|<.*>|none|null|default)$`)
	// A value that names where a secret lives is a reference, not the secret.
	secretReference = regexp.MustCompile(`(?i)^\$|^<|/secrets/|secretmanager|arn:aws[a-z\-]*:secretsmanager|vault\.azure\.net|^vault:|^env\(|^\*+$`)
	placeholder     = regexp.MustCompile(`\$\{([A-Za-z_][\w.\-]*)(?::[^}]*)?\}|\$\(([A-Za-z_]\w*)\)|\$([A-Za-z_]\w*)`)
	lineKV          = regexp.MustCompile(`^\s*(?:export\s+)?["']?([A-Za-z_][\w.\-/]*)["']?\s*[:=]\s*(.+?)\s*[,;\\]?\s*$`)
	cliFlag         = regexp.MustCompile(`--([A-Za-z][\w\-]*)[= ]["']?([^\s"'\\]+)`)
	xmlText         = regexp.MustCompile(`<([A-Za-z][\w.\-:]*)>([^<>]{2,300})</([A-Za-z][\w.\-:]*)>`)
	hclHeader       = regexp.MustCompile(`^\s*(resource|module|data|variable|output|locals)\s*(?:"([^"]+)")?\s*(?:"([^"]+)")?\s*\{`)
	hclSource       = regexp.MustCompile(`^\s*source\s*=\s*"([^"]+)"`)
	hclLiteral      = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
	sqlDDL          = regexp.MustCompile(`(?i)\bcreate\s+(?:or\s+replace\s+)?(table|view|schema|dataset)\s+(?:if\s+not\s+exists\s+)?[` + "`" + `"\[]?([\w.\-${}]+)`)
	scalarNoise     = regexp.MustCompile(`(?i)^(true|false|yes|no|null|none|~|\d+(\.\d+)?[a-z]{0,3}|\{|\[|\||>|-)$`)
)

// HCL meta-arguments are language syntax, not values.
var hclMeta = map[string]bool{"source": true, "depends_on": true, "count": true, "for_each": true, "provider": true, "providers": true, "lifecycle": true, "version": true}

func keepValue(v string) (string, bool) {
	v = strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"'`))
	if len(v) < 2 || len(v) > 300 || scalarNoise.MatchString(v) {
		return "", false
	}
	return v, true
}

func lastKey(keyPath string) string {
	parts := strings.FieldsFunc(strings.TrimRight(keyPath, "]"), func(r rune) bool { return r == '.' || r == '[' || r == ']' || r == '/' })
	if len(parts) == 0 {
		return keyPath
	}
	return parts[len(parts)-1]
}

func flatten(v any, prefix string, emit func(string, string)) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flatten(x[k], p, emit)
		}
	case []any:
		for i, item := range x {
			// Kubernetes-style environment lists: {name: KEY, value: VALUE}
			if m, ok := item.(map[string]any); ok {
				if n, ok := m["name"].(string); ok {
					if val, ok := m["value"]; ok {
						if s, ok := scalar(val); ok {
							emit(prefix+"."+n, s)
							continue
						}
					}
				}
			}
			flatten(item, fmt.Sprintf("%s[%d]", prefix, i), emit)
		}
	default:
		if s, ok := scalar(v); ok {
			emit(prefix, s)
		}
	}
}

func scalar(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case nil, bool:
		return "", false
	default:
		return fmt.Sprint(x), true
	}
}

func parseConfig(file string, data []byte) []entry {
	out := []entry{}
	ext := strings.ToLower(path.Ext(file))
	add := func(keyPath, value, ctx string) {
		v, ok := keepValue(value)
		if !ok || keyPath == "" {
			return
		}
		out = append(out, entry{File: file, KeyPath: keyPath, Key: lastKey(keyPath), Value: v, Context: ctx})
	}
	switch ext {
	case ".json":
		var v any
		if json.Unmarshal(data, &v) == nil {
			flatten(v, "", func(k, s string) { add(k, s, "") })
			return out
		}
	case ".yaml", ".yml":
		if parseYAML(data, add) {
			return out
		}
	case ".sql":
		for _, m := range sqlDDL.FindAllStringSubmatch(string(data), -1) {
			add("ddl."+strings.ToLower(m[1]), m[2], "sql-ddl")
		}
		return out
	}
	hcl := ext == ".tf" || ext == ".tfvars" || ext == ".hcl"
	ctx := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		if hcl {
			if m := hclHeader.FindStringSubmatch(line); m != nil {
				ctx = strings.TrimSpace(strings.Join([]string{m[1], m[2], m[3]}, " "))
				continue
			}
			if m := hclSource.FindStringSubmatch(line); m != nil {
				ctx = strings.SplitN(ctx, " source=", 2)[0] + " source=" + m[1]
				continue
			}
		}
		for _, m := range cliFlag.FindAllStringSubmatch(line, -1) {
			add(m[1], m[2], "cli-flag")
		}
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		if m := lineKV.FindStringSubmatch(line); m != nil {
			if hcl {
				if hclMeta[m[1]] {
					continue
				}
				expr := m[2]
				if i := strings.Index(expr, "?"); i >= 0 { // conditional: only the branches are values
					expr = expr[i+1:]
				}
				seen := map[string]bool{}
				for _, lit := range hclLiteral.FindAllStringSubmatch(expr, -1) {
					if !seen[lit[1]] {
						seen[lit[1]] = true
						add(m[1], lit[1], ctx)
					}
				}
			} else {
				add(m[1], m[2], ctx)
			}
		}
		for _, m := range xmlText.FindAllStringSubmatch(line, -1) {
			if m[1] == m[3] {
				add(m[1], m[2], "xml")
			}
		}
	}
	return out
}

func parseYAML(data []byte, add func(string, string, string)) bool {
	collected := []entry{}
	collect := func(k, v, ctx string) {
		if s, ok := keepValue(v); ok && k != "" {
			collected = append(collected, entry{KeyPath: k, Key: lastKey(k), Value: s, Context: ctx})
		}
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc any
		e := dec.Decode(&doc)
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return false
		}
		kind := ""
		if m, ok := doc.(map[string]any); ok {
			kind, _ = m["kind"].(string)
		}
		flatten(doc, "", func(k, s string) { collect(k, s, kind) })
	}
	for _, c := range collected {
		add(c.KeyPath, c.Value, c.Context)
	}
	return true
}

func isText(b []byte) bool {
	n := len(b)
	if n > 4096 {
		n = 4096
	}
	return !bytes.Contains(b[:n], []byte{0})
}

func scanConfig(s *snapshot) ([]entry, int, error) {
	out := []entry{}
	files := 0
	for _, f := range s.files {
		base := path.Base(f)
		if nonConfigExt[strings.ToLower(path.Ext(f))] || testConfig.MatchString(f) || nonRuntime.MatchString(f) || strings.Contains(f, "node_modules/") {
			continue
		}
		switch base {
		case "package.json", "package-lock.json", "go.mod", "go.sum", "LICENSE", ".gitignore", ".dockerignore", ".gcloudignore", ".DS_Store":
			continue
		}
		b, e := s.read(f)
		if e != nil {
			return nil, files, e
		}
		if b == nil || len(b) > 512_000 || !isText(b) {
			continue
		}
		files++
		secret := secretFile.MatchString(f)
		for _, x := range parseConfig(f, b) {
			x.File = f
			for _, m := range placeholder.FindAllStringSubmatch(x.Value, -1) {
				for _, g := range m[1:] {
					if g != "" {
						x.Placeholders = append(x.Placeholders, g)
						break
					}
				}
			}
			if credentialEntry(x.Key, x.Value) {
				x.Credential = true
			}
			if secret || x.Credential {
				x.Value, x.Secret = "<redacted>", true
			}
			out = append(out, x)
		}
	}
	return out, files, nil
}

// credentialEntry reports whether a configuration entry holds a credential: a value carrying a signature,
// token, password or key, or a literal under a secret-named key (not a name, path or URL of the secret).
func credentialEntry(key, value string) bool {
	v := strings.TrimSpace(value)
	if v == "" || secretReference.MatchString(v) {
		return false
	}
	if credentialValue.MatchString(v) {
		return true
	}
	return credentialKey.MatchString(key) && !secretPointer.MatchString(key) && len(v) >= 6 && !exampleValue.MatchString(strings.Trim(v, `"'`)) &&
		!strings.ContainsAny(v, " \t") && !strings.Contains(v, "://") && !strings.HasPrefix(v, "/")
}
