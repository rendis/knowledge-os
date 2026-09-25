package discover

import (
	_ "embed"
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Language rules only: how an import is written, how own code is told apart from
// external code, and which modules belong to the language runtime. No framework lists.

//go:embed langdata/node_builtins.txt
var nodeBuiltinsText string

//go:embed langdata/python_stdlib.txt
var pythonStdlibText string

var nodeBuiltins = lines(nodeBuiltinsText)
var pythonStdlib = lines(pythonStdlibText)

func lines(s string) map[string]bool {
	m := map[string]bool{}
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			m[l] = true
		}
	}
	return m
}

var codeExt = map[string]string{".go": "go", ".java": "java", ".kt": "java", ".ts": "js", ".tsx": "js", ".js": "js", ".jsx": "js", ".mjs": "js", ".cjs": "js", ".py": "py"}

// Tests, vendored and generated trees are excluded from the evidence surface.
var skipCode = regexp.MustCompile(`(^|/)(vendor|node_modules|dist|build|target|coverage|__pycache__|\.venv|venv|\.next|www|public)/|_test\.go$|\.(spec|test)\.[jt]sx?$|(^|/)(tests?|__tests__|mocks?|testdata)/|(^|/)test_[^/]+\.py$|\.d\.ts$|\.min\.js$|\.config\.[jt]s$`)

var (
	goBlock   = regexp.MustCompile(`(?ms)^import\s*\((.*?)^\)`)
	goSingle  = regexp.MustCompile(`(?m)^import\s+(?:[\w.]+\s+)?("[^"]+")`)
	quoted    = regexp.MustCompile(`"([^"]+)"`)
	javaImp   = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([\w.*]+)\s*;?\s*$`)
	jsImp     = regexp.MustCompile(`(?m)^\s*(?:import|export)\b[^;'"]*?\bfrom\s*['"]([^'"]+)['"]|^\s*import\s*['"]([^'"]+)['"]|\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)|\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	jsFetch   = regexp.MustCompile(`(?:^|[^\w.])fetch\s*\(`)
	pyImp     = regexp.MustCompile(`(?m)^\s*(?:from\s+([\w.]+)\s+import|import\s+([\w.,\s]+?)\s*(?:#.*)?$)`)
	javaPkg   = regexp.MustCompile(`(?m)^package\s+([\w.]+)`)
	goModule  = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	goRequire = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([\w.\-]+\.[\w.\-]+/[\w./\-]+)\s+v[\d.][^\n]*$`)
	pomDep    = regexp.MustCompile(`(?s)<dependency>\s*<groupId>([^<]+)</groupId>\s*<artifactId>([^<]+)</artifactId>`)
	gradleDep = regexp.MustCompile(`(?:implementation|api|compile|runtimeOnly|compileOnly)\s*\(?\s*['"]([\w.\-]+:[\w.\-]+)`)
	pyReq     = regexp.MustCompile(`(?m)^\s*"?([A-Za-z][\w.\-]*)`)
)

func imports(lang, src string) []string {
	out := []string{}
	switch lang {
	case "go":
		blocks := []string{}
		for _, m := range goBlock.FindAllStringSubmatch(src, -1) {
			blocks = append(blocks, m[1])
		}
		for _, m := range goSingle.FindAllStringSubmatch(src, -1) {
			blocks = append(blocks, m[1])
		}
		for _, m := range quoted.FindAllStringSubmatch(strings.Join(blocks, "\n"), -1) {
			out = append(out, m[1])
		}
	case "java":
		for _, m := range javaImp.FindAllStringSubmatch(src, -1) {
			out = append(out, m[1])
		}
	case "js":
		for _, m := range jsImp.FindAllStringSubmatch(src, -1) {
			for _, g := range m[1:] {
				if g != "" {
					out = append(out, g)
				}
			}
		}
	case "py":
		for _, m := range pyImp.FindAllStringSubmatch(src, -1) {
			if m[1] != "" {
				out = append(out, m[1])
				continue
			}
			for _, x := range strings.Split(m[2], ",") {
				if x = strings.TrimSpace(strings.SplitN(strings.TrimSpace(x), " as ", 2)[0]); x != "" {
					out = append(out, x)
				}
			}
		}
	}
	return out
}

// manifest holds what the build files declare; it decides what counts as external.
type manifest struct {
	goModules  []string
	goRequires []string
	npm        map[string]bool
	npmName    string
	declared   []string // direct dependencies as declared (for declared-but-not-imported checks)
}

func readManifest(s *snapshot) (manifest, error) {
	m := manifest{npm: map[string]bool{}}
	seen := map[string]bool{}
	add := func(d string) {
		if d != "" && !seen[d] {
			seen[d] = true
			m.declared = append(m.declared, d)
		}
	}
	for _, f := range s.files {
		base := path.Base(f)
		if strings.Contains(f, "node_modules/") || skipCode.MatchString(f) && base != "go.mod" {
			continue
		}
		switch base {
		case "go.mod", "package.json", "pom.xml", "build.gradle", "build.gradle.kts", "requirements.txt", "pyproject.toml":
		default:
			continue
		}
		b, e := s.read(f)
		if e != nil {
			return m, e
		}
		t := string(b)
		switch base {
		case "go.mod":
			if mm := goModule.FindStringSubmatch(t); mm != nil {
				m.goModules = append(m.goModules, mm[1])
			}
			for _, r := range goRequire.FindAllStringSubmatch(t, -1) {
				m.goRequires = append(m.goRequires, r[1])
				if !strings.Contains(r[0], "// indirect") {
					add("go:" + r[1])
				}
			}
		case "package.json":
			var j map[string]any
			if json.Unmarshal(b, &j) == nil {
				if n, _ := j["name"].(string); n != "" && m.npmName == "" {
					m.npmName = n
				}
				for _, k := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
					deps, _ := j[k].(map[string]any)
					for d := range deps {
						m.npm[d] = true
						if k == "dependencies" {
							add("js:" + d)
						}
					}
				}
			}
		case "pom.xml":
			for _, d := range pomDep.FindAllStringSubmatch(t, -1) {
				add("maven:" + strings.TrimSpace(d[1]) + ":" + strings.TrimSpace(d[2]))
			}
		case "build.gradle", "build.gradle.kts":
			for _, d := range gradleDep.FindAllStringSubmatch(t, -1) {
				add("maven:" + d[1])
			}
		default:
			for _, d := range pyReq.FindAllStringSubmatch(t, -1) {
				n := strings.ToLower(strings.ReplaceAll(d[1], "-", "_"))
				switch n {
				case "project", "name", "version", "dependencies", "requires", "build", "tool", "description", "readme", "authors", "license":
					continue
				}
				add("py:" + n)
			}
		}
	}
	return m, nil
}

// dependency is one external or runtime module family reached from a repository.
type dependency struct {
	ID       string   `json:"id"`
	Lang     string   `json:"lang"`
	Origin   string   `json:"origin"` // import | manifest | library
	Via      string   `json:"via,omitempty"`
	Files    []string `json:"files,omitempty"`
	Paths    []string `json:"imported_paths,omitempty"`
	Category string   `json:"category,omitempty"`
}

type classified struct{ kind, family string }

type langContext struct {
	m        manifest
	javaOwn  string
	pyLocal  map[string]bool
	goLocals map[string]string // module path -> local repository (company libraries)
}

func classifyImport(lang, spec string, c langContext) classified {
	switch lang {
	case "go":
		for _, mod := range c.m.goModules {
			if spec == mod || strings.HasPrefix(spec, mod+"/") {
				return classified{"own", ""}
			}
		}
		first := strings.SplitN(spec, "/", 2)[0]
		if !strings.Contains(first, ".") {
			return classified{"std", spec}
		}
		best := ""
		for _, r := range c.m.goRequires {
			if (spec == r || strings.HasPrefix(spec, r+"/")) && len(r) > len(best) {
				best = r
			}
		}
		if best == "" {
			parts := strings.Split(spec, "/")
			if len(parts) > 3 {
				parts = parts[:3]
			}
			best = strings.Join(parts, "/")
		}
		return classified{"ext", best}
	case "java":
		keep := []string{}
		for i, s := range strings.Split(spec, ".") {
			if i >= 3 || s == "*" || (s != "" && strings.ToUpper(s[:1]) == s[:1] && strings.ToLower(s[:1]) != s[:1]) {
				break
			}
			keep = append(keep, s)
		}
		fam := strings.Join(keep, ".")
		if strings.HasPrefix(spec, "java.") || strings.HasPrefix(spec, "javax.") || strings.HasPrefix(spec, "jdk.") || strings.HasPrefix(spec, "kotlin.") {
			return classified{"std", fam}
		}
		if c.javaOwn != "" && strings.HasPrefix(spec, c.javaOwn) {
			return classified{"own", ""}
		}
		return classified{"ext", fam}
	case "js":
		s := strings.TrimPrefix(spec, "node:")
		root := strings.SplitN(s, "/", 2)[0]
		if nodeBuiltins[root] {
			return classified{"std", root}
		}
		pkg := root
		if strings.HasPrefix(s, "@") {
			parts := strings.SplitN(s, "/", 3)
			if len(parts) >= 2 {
				pkg = parts[0] + "/" + parts[1]
			}
		}
		if c.m.npm[pkg] {
			return classified{"ext", pkg}
		}
		return classified{"own", ""} // undeclared: alias, asset or own code
	case "py":
		if strings.HasPrefix(spec, ".") {
			return classified{"own", ""}
		}
		root := strings.SplitN(spec, ".", 2)[0]
		if c.pyLocal[root] {
			return classified{"own", ""}
		}
		if pythonStdlib[root] {
			return classified{"std", root}
		}
		return classified{"ext", root}
	}
	return classified{"own", ""}
}

// codeSurface is what the source code of one snapshot imports, per file.
type codeSurface struct {
	Languages map[string]int         `json:"languages"`
	Files     map[string][]importRef `json:"-"`
	Manifest  manifest               `json:"-"`
	Literals  map[string][]string    `json:"-"` // quoted identifier-like literals -> files
}

var codeLiteral = regexp.MustCompile("[\"'`]([A-Za-z0-9][\\w.\\-/:]{3,150})[\"'`]")

type importRef struct{ Kind, Family, Spec string }

func scanCode(s *snapshot, goLocals map[string]string) (codeSurface, error) {
	cs := codeSurface{Languages: map[string]int{}, Files: map[string][]importRef{}, Literals: map[string][]string{}}
	m, e := readManifest(s)
	if e != nil {
		return cs, e
	}
	cs.Manifest = m
	ctx := langContext{m: m, pyLocal: map[string]bool{}, goLocals: goLocals}
	pkgs := map[string]int{}
	for _, f := range s.files {
		lang := codeExt[path.Ext(f)]
		if lang == "py" {
			for _, p := range strings.Split(f, "/") {
				ctx.pyLocal[strings.TrimSuffix(p, ".py")] = true
			}
		}
	}
	type src struct{ file, lang, text string }
	sources := []src{}
	for _, f := range s.files {
		lang := codeExt[path.Ext(f)]
		if lang == "" || skipCode.MatchString(f) {
			continue
		}
		b, e := s.read(f)
		if e != nil {
			return cs, e
		}
		t := string(b)
		if lang == "java" {
			if mm := javaPkg.FindStringSubmatch(t); mm != nil {
				parts := strings.Split(mm[1], ".")
				if len(parts) > 3 {
					parts = parts[:3]
				}
				pkgs[strings.Join(parts, ".")]++
			}
		}
		sources = append(sources, src{f, lang, t})
	}
	best := 0
	for p, n := range pkgs {
		if n > best || n == best && p < ctx.javaOwn {
			ctx.javaOwn, best = p, n
		}
	}
	for _, x := range sources {
		cs.Languages[x.lang]++
		refs := []importRef{}
		if x.lang == "js" && jsFetch.MatchString(x.text) {
			refs = append(refs, importRef{"std", "fetch", "fetch"})
		}
		for _, spec := range imports(x.lang, x.text) {
			c := classifyImport(x.lang, spec, ctx)
			if c.kind != "own" {
				refs = append(refs, importRef{c.kind, c.family, spec})
			}
		}
		cs.Files[x.file] = refs
		for _, m := range codeLiteral.FindAllStringSubmatch(x.text, -1) {
			cs.Literals[m[1]] = appendUnique(cs.Literals[m[1]], x.file)
		}
	}
	return cs, nil
}

func dependencyID(lang, family string) string { return lang + ":" + family }

// directDependencies aggregates import families per repository (before library resolution).
func directDependencies(cs codeSurface) map[string]*dependency {
	out := map[string]*dependency{}
	for f, refs := range cs.Files {
		lang := codeExt[path.Ext(f)]
		for _, r := range refs {
			id := dependencyID(lang, r.Family)
			d := out[id]
			if d == nil {
				d = &dependency{ID: id, Lang: lang, Origin: "import"}
				out[id] = d
			}
			d.Files = appendUnique(d.Files, f)
			d.Paths = appendUnique(d.Paths, r.Spec)
		}
	}
	for _, d := range out {
		sort.Strings(d.Files)
		sort.Strings(d.Paths)
	}
	return out
}

func appendUnique(xs []string, x string) []string {
	for _, y := range xs {
		if y == x {
			return xs
		}
	}
	return append(xs, x)
}
