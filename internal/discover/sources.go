package discover

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Anchor is one source permalink a note cites: owner/repo, commit, path and optional line range.
type Anchor struct {
	Repo   string // owner/name
	Commit string
	Path   string
	From   int
	To     int
}

// Anchors returns the permalinks written in text, in order.
func Anchors(text string) []Anchor {
	out := []Anchor{}
	for _, a := range parseAnchors(text) {
		out = append(out, Anchor{Repo: a.Repo, Commit: a.Commit, Path: a.Path, From: a.From, To: a.To})
	}
	return out
}

// AnchorState says whether a cited file still reads the same on the repository's reference branch.
type AnchorState struct {
	Status string // current | changed | unknown
	Ref    string // the reference branch it was compared with
	Detail string // why it is unknown
}

// Sources reads cited files from the local checkouts and compares them with the reference branch,
// caching per repository and commit so that one pack checks many anchors with a few git calls.
type Sources struct {
	vault   string
	repos   map[string]*sourceRepo
	changed map[string]map[string]bool // repo path + commit → files changed up to the reference head
	hunks   map[string][][2]int
	files   *gitCache
}

type sourceRepo struct {
	path, ref, head, err string
}

// NewSources binds a reader to the vault's workspace.
func NewSources(vault string) *Sources {
	return &Sources{vault: vault, repos: map[string]*sourceRepo{}, changed: map[string]map[string]bool{}, hunks: map[string][][2]int{}, files: &gitCache{commits: map[string]bool{}, files: map[string]*string{}}}
}

func (s *Sources) repo(ownerRepo string) *sourceRepo {
	name := ownerRepo
	if i := strings.Index(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if r, ok := s.repos[strings.ToLower(name)]; ok {
		return r
	}
	r := &sourceRepo{}
	s.repos[strings.ToLower(name)] = r
	inputs, e := discoverRepositories(s.vault, map[string]bool{name: true})
	switch {
	case e != nil || len(inputs) != 1:
		r.err = name + " is not a tracked repository of this vault"
	case inputs[0].Path == "":
		r.err = "no local checkout of " + name
	case inputs[0].RefErr != "":
		r.path, r.err = inputs[0].Path, inputs[0].RefErr
	default:
		r.path, r.ref = inputs[0].Path, inputs[0].Ref
		if h, e := resolveCommit(r.path, r.ref); e == nil && h != "" {
			r.head = h
		} else {
			r.err = "reference " + r.ref + " does not resolve in " + r.path
		}
	}
	return r
}

// Checkout returns the local checkout of a cited repository, or "".
func (s *Sources) Checkout(ownerRepo string) string {
	return s.repo(ownerRepo).path
}

// State compares the cited file at its commit with the reference branch.
func (s *Sources) State(a Anchor) AnchorState {
	r := s.repo(a.Repo)
	if r.err != "" {
		return AnchorState{Status: "unknown", Detail: r.err}
	}
	short := shortRef(r.ref)
	full, e := resolveCommit(r.path, a.Commit)
	if e != nil || full == "" {
		return AnchorState{Status: "unknown", Ref: short, Detail: "commit " + a.Commit + " is not in the local checkout; fetch it"}
	}
	if full == r.head {
		return AnchorState{Status: "current", Ref: short}
	}
	key := r.path + "\x00" + full
	set, ok := s.changed[key]
	if !ok {
		set = map[string]bool{}
		out, _ := gitOutput(r.path, "diff", "--name-only", full, r.head)
		for _, f := range strings.Split(out, "\n") {
			if f != "" {
				set[f] = true
			}
		}
		s.changed[key] = set
	}
	if !set[a.Path] {
		return AnchorState{Status: "current", Ref: short}
	}
	// The file changed; the claim is stale only when the change touches the cited lines.
	hk := key + "\x00" + a.Path
	hunks, ok := s.hunks[hk]
	if !ok {
		hunks = changedLines(r.path, full, r.head, a.Path)
		s.hunks[hk] = hunks
	}
	if overlaps(hunks, a.From, a.To) {
		return AnchorState{Status: "changed", Ref: short}
	}
	return AnchorState{Status: "current", Ref: short, Detail: "the file changed outside the cited lines"}
}

// Symbol finds, in the cited file at its commit, where the first of names is defined (a function,
// method, type or configuration key; else its first mention) and returns up to max lines from there
// with that line number and the name found.
func (s *Sources) Symbol(a Anchor, names []string, max int) (string, int, string, bool) {
	r := s.repo(a.Repo)
	if r.path == "" || !s.files.commit(r.path, a.Commit) {
		return "", 0, "", false
	}
	content, ok := s.files.file(r.path, a.Commit, a.Path)
	if !ok {
		return "", 0, "", false
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for _, name := range names {
		q := regexp.QuoteMeta(name)
		for _, re := range []*regexp.Regexp{
			regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s*)?` + q + `\s*[\[(]`),
			regexp.MustCompile(`\b(?:def|class|function|interface|type|struct|enum|record)\s+` + q + `\b`),
			regexp.MustCompile(`\b` + q + `\s*[:=]\s*(?:async\s+)?(?:function\b|\(|func\b)`),
			regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|final|async|override|export)\s+)*[\w<>\[\],.?]+\s+` + q + `\s*\(`),
			regexp.MustCompile(`^\s*["']?` + q + `["']?\s*[:=]`),
			regexp.MustCompile(`\b` + q + `\b`),
		} {
			for n, l := range lines {
				if re.MatchString(l) {
					to := min(n+max, len(lines))
					return strings.Join(lines[n:to], "\n"), n + 1, name, true
				}
			}
		}
	}
	return "", 0, "", false
}

// Hit is one line of the reference branch that names a searched identifier.
type Hit struct {
	Path string
	Line int
	Text string
	In   string // the enclosing function, when known
}

// Grep searches the repository's reference branch for each name as a whole word, tests and Markdown
// left out, and returns up to per hits for each name found, with the branch it read.
func (s *Sources) Grep(ownerRepo string, names []string, per int) (map[string][]Hit, map[string][]Hit, string) {
	r := s.repo(ownerRepo)
	out, all := map[string][]Hit{}, map[string][]Hit{}
	if r.head == "" || len(names) == 0 {
		return out, all, ""
	}
	args := []string{"grep", "-n", "-I", "-w", "-F"}
	for _, n := range names {
		args = append(args, "-e", n)
	}
	args = append(append(args, r.head, "--", ".", ":(exclude)*.md"), excluded...)
	text, _ := gitOutput(r.path, args...)
	for _, l := range strings.Split(text, "\n") {
		parts := strings.SplitN(l, ":", 4)
		if len(parts) != 4 {
			continue
		}
		n, e := strconv.Atoi(parts[2])
		if e != nil {
			continue
		}
		// A line can name several of them (eventType := "orderAcknowledged"): it counts for each.
		for _, name := range names {
			if wholeWord(parts[3], name) {
				out[name] = append(out[name], Hit{Path: parts[1], Line: n, Text: Redact(strings.TrimSpace(parts[3]))})
			}
		}
	}
	// Code before configuration, production configuration before the rest: where a name is used,
	// then the value it runs with.
	rank := func(p string) int {
		switch {
		case codeFile(p):
			return 0
		case strings.Contains(p, "prod"):
			return 1
		}
		return 2
	}
	for name, hs := range out {
		all[name] = append([]Hit{}, hs...)
		sort.SliceStable(hs, func(a, b int) bool { return rank(hs[a].Path) < rank(hs[b].Path) })
		// One line per file first: where it is declared, where it is used, what value it runs with.
		picked, files, rest := []Hit{}, map[string]bool{}, []Hit{}
		for _, h := range hs {
			if !files[h.Path] && len(picked) < per {
				files[h.Path] = true
				picked = append(picked, h)
			} else {
				rest = append(rest, h)
			}
		}
		for _, h := range rest {
			if len(picked) == per {
				break
			}
			picked = append(picked, h)
		}
		// The function each picked line is in: where a name is produced, checked or used.
		for k := range picked {
			if codeFile(picked[k].Path) {
				if content, ok := s.files.file(r.path, r.head, picked[k].Path); ok {
					src := strings.Split(content, "\n")
					for x := min(picked[k].Line, len(src)) - 1; x >= 0; x-- {
						if isFuncStart(src[x]) {
							picked[k].In = definedName(src[x])
							break
						}
					}
				}
			}
		}
		out[name] = picked
	}
	return out, all, shortRef(r.ref)
}

// shortRef is the ref as git accepts it in a command: origin/main for a remote-tracking branch.
func shortRef(ref string) string {
	return strings.TrimPrefix(strings.TrimPrefix(ref, "refs/remotes/"), "refs/heads/")
}

// Function is a stretch of code around the lines that hold the searched words: the enclosing
// function when one is recognizable.
type Function struct {
	Path, Name string
	From, To   int
	Code       string // numbered lines
	Words      []string
	Outside    []int // matching lines of the same function beyond the lines shown
	Callers    []Hit // where the function is called from, outside its own file's definition
}

// linesToShow picks at most span lines: the whole function when its matches fit from its first
// line, otherwise each block of nearby matches with two lines around it, the blocks with the most
// matches first, returned in file order.
func linesToShow(start int, hits []int, span, total int) []int {
	if len(hits) == 0 {
		return nil
	}
	last := hits[len(hits)-1]
	if start > 0 && last-start+3 <= span {
		out := []int{}
		for k := start; k <= min(last+2, total); k++ {
			out = append(out, k)
		}
		return out
	}
	type block struct{ from, to, n int }
	blocks := []block{}
	for _, h := range hits {
		if len(blocks) > 0 && h-blocks[len(blocks)-1].to <= 6 {
			blocks[len(blocks)-1].to, blocks[len(blocks)-1].n = h, blocks[len(blocks)-1].n+1
			continue
		}
		blocks = append(blocks, block{h, h, 1})
	}
	sort.SliceStable(blocks, func(a, b int) bool { return blocks[a].n > blocks[b].n })
	set := map[int]bool{}
	for _, b := range blocks {
		add := []int{}
		for k := max(1, b.from-2); k <= min(total, b.to+2); k++ {
			if !set[k] {
				add = append(add, k)
			}
		}
		if len(set)+len(add) > span {
			if len(set) > 0 {
				continue
			}
			add = add[:span]
		}
		for _, k := range add {
			set[k] = true
		}
	}
	out := []int{}
	for k := range set {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

var definedNameRe = regexp.MustCompile(`(?:\bfunc\s+(?:\([^)]*\)\s*)?|\bdef\s+|\bfunction\s+|^\s*(?:(?:public|private|protected|internal|static|override|async)\s+)*[\w<>\[\],.?]+\s+)([A-Za-z_]\w*)\s*[\[(]|^\s*(?:export\s+)?(?:const|let|var)?\s*([A-Za-z_]\w*)\s*[:=]\s*(?:async\s+)?(?:function\b|\()`)

// DefinedName is the function a definition line declares.
func DefinedName(line string) string { return definedName(line) }

// definedName is the function a definition line declares.
func definedName(line string) string {
	m := definedNameRe.FindStringSubmatch(line)
	if m == nil {
		if mm := methodStart.FindStringSubmatch(line); mm != nil && !controlWord[mm[1]] {
			return mm[1]
		}
		if mm := methodOpen.FindStringSubmatch(line); mm != nil && !controlWord[mm[1]] {
			return mm[1]
		}
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// calledThroughRival: every use of name on the line means another package's function with that
// name: qualified by that package, or unqualified inside that package's own directory.
func calledThroughRival(line, name, own string, rivals map[string]bool, inRivalDir bool) bool {
	if len(rivals) == 0 {
		return false
	}
	uses := regexp.MustCompile(`(\w+\.)?\b`+regexp.QuoteMeta(name)+`\b`).FindAllStringSubmatch(line, -1)
	for _, u := range uses {
		q := strings.TrimSuffix(u[1], ".")
		switch {
		case q == "" && inRivalDir, rivals[q] && q != own:
		default:
			return false
		}
	}
	return len(uses) > 0
}

// importLine brings a name in or exports it; neither calls it.
var importLine = regexp.MustCompile(`^(import\b|export\s+(default\s+)?[\w{]|from\s+\S+\s+import\b|module\.exports\b|exports\.\w+\s*=|(const|let|var)\s+[\w{},\s]+=\s*require\()`)

// interfaceMethod is a method declared without a body: Handle(confirmed domain.TransactionConfirmed) error.
var interfaceMethod = regexp.MustCompile(`^[A-Za-z_]\w*\s*\(\s*(?:\w+\s+[\w.*\[\]]+\s*,?\s*)*\)\s*[\w.*\[\](), ]*$`)

// callers lists where name is called in the code of the reference branch, definitions left out.
func (s *Sources) callers(r *sourceRepo, name, defFile string, n int) []Hit {
	text, _ := gitOutput(r.path, append([]string{"grep", "-n", "-I", "-w", "-F", "-e", name, r.head, "--", "."}, excluded...)...)
	// Other packages defining a function with this name: a call qualified by one of them
	// (otherpkg.GetID) is not a call of the one in defFile.
	rivals, rivalDirs := map[string]bool{}, map[string]bool{}
	for _, l := range strings.Split(text, "\n") {
		if parts := strings.SplitN(l, ":", 4); len(parts) == 4 && definedName(parts[3]) == name && filepath.Dir(parts[1]) != filepath.Dir(defFile) {
			rivals[filepath.Base(filepath.Dir(parts[1]))] = true
			rivalDirs[filepath.Dir(parts[1])] = true
		}
	}
	own := filepath.Base(filepath.Dir(defFile))
	out := []Hit{}
	for _, l := range strings.Split(text, "\n") {
		parts := strings.SplitN(l, ":", 4)
		// A call, or the function passed as a value (processPages(…, sendPage)); not
		// its definition, not a comment.
		if len(parts) != 4 {
			continue
		}
		t := strings.TrimSpace(parts[3])
		if !codeFile(parts[1]) || definedName(parts[3]) == name || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") || interfaceMethod.MatchString(t) || importLine.MatchString(t) {
			continue
		}
		if defFile != "" && calledThroughRival(t, name, own, rivals, rivalDirs[filepath.Dir(parts[1])]) {
			continue
		}
		line, _ := strconv.Atoi(parts[2])
		h := Hit{Path: parts[1], Line: line, Text: Redact(strings.TrimSpace(parts[3]))}
		if content, ok := s.files.file(r.path, r.head, h.Path); ok {
			src := strings.Split(content, "\n")
			for k := min(line, len(src)) - 2; k >= 0; k-- {
				if isFuncStart(src[k]) {
					h.In = definedName(src[k])
					break
				}
			}
		}
		out = append(out, h)
		if len(out) == n {
			break
		}
	}
	return out
}

func uniqueInts(xs []int) []int {
	out := []int{}
	for _, x := range xs {
		if len(out) == 0 || out[len(out)-1] != x {
			out = append(out, x)
		}
	}
	return out
}

// isFuncStart reports whether a line opens a function or method: a keyword form (func, def,
// function, a modifier), an assigned function, or a class method written as name(args) { — the
// last one only when the name is not a control keyword (if, for, while…).
func isFuncStart(line string) bool {
	if funcStart.MatchString(line) {
		return true
	}
	m := methodStart.FindStringSubmatch(line)
	if m == nil {
		m = methodOpen.FindStringSubmatch(line) // a signature continued on the next lines
	}
	return m != nil && !controlWord[m[1]]
}

var methodOpen = regexp.MustCompile(`^\s*(?:async\s+|static\s+|public\s+|private\s+|protected\s+|override\s+)*([A-Za-z_$][\w$]*)\s*\($`)

var methodStart = regexp.MustCompile(`^\s*(?:async\s+|get\s+|set\s+|static\s+)*([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\([^;]*\)\s*(?::\s*[^={;]+)?\{\s*$`)

var controlWord = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "else": true, "function": true, "do": true, "try": true, "with": true, "foreach": true, "synchronized": true, "using": true, "lock": true, "elif": true, "unless": true}

var funcStart = regexp.MustCompile(`^\s*(?:func\b|def\b|(?:export\s+)?(?:async\s+)?function\b|(?:public|private|protected|internal|static|override)\b[^;=]*\(|(?:(?:export\s+)?(?:const|let|var)\s+)?[\w.]+\s*[:=]\s*(?:async\s+)?(?:function\b|\([^)]*\)\s*=>))`)

// Functions searches the code files of the reference branch for the words (case-insensitive
// substrings, tests and comments left out) and returns up to n functions holding the most distinct
// words, those in prefer first, each at most span lines.
func (s *Sources) Functions(ownerRepo string, words []string, n, span int, prefer map[string]bool) ([]Function, string) {
	r := s.repo(ownerRepo)
	if r.head == "" || len(words) == 0 {
		return nil, ""
	}
	quoted := []string{}
	for _, w := range words {
		quoted = append(quoted, regexp.QuoteMeta(w))
	}
	text, _ := gitOutput(r.path, append([]string{"grep", "-n", "-I", "-i", "-E", strings.Join(quoted, "|"), r.head, "--", "."}, excluded...)...)
	type fn struct {
		f     Function
		lines []int
		words map[string]bool
	}
	found := map[string]*fn{}
	files := map[string][]string{}
	for _, l := range strings.Split(text, "\n") {
		parts := strings.SplitN(l, ":", 4)
		if len(parts) != 4 || !codeFile(parts[1]) {
			continue
		}
		line, e := strconv.Atoi(parts[2])
		if e != nil {
			continue
		}
		if t := strings.TrimSpace(parts[3]); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			continue // commented-out code and comments are not what the code does
		}
		path := parts[1]
		src, ok := files[path]
		if !ok {
			content, _ := s.files.file(r.path, r.head, path)
			src = strings.Split(content, "\n")
			files[path] = src
		}
		start := 0
		for k := min(line, len(src)) - 1; k >= 0; k-- {
			if isFuncStart(src[k]) {
				start = k + 1
				break
			}
		}
		key := fmt.Sprintf("%s:%d", path, start)
		f := found[key]
		if f == nil {
			f = &fn{f: Function{Path: path, From: start}, words: map[string]bool{}}
			if start > 0 {
				f.f.Name = strings.TrimSpace(src[start-1])
			}
			found[key] = f
		}
		f.lines = append(f.lines, line)
		lower := strings.ToLower(parts[3])
		for _, w := range words {
			if strings.Contains(lower, strings.ToLower(w)) {
				f.words[w] = true
			}
		}
	}
	list := []*fn{}
	for _, f := range found {
		list = append(list, f)
	}
	sort.Slice(list, func(a, b int) bool {
		// The files the note's best paragraphs cite come first: the code the claim is about.
		if pa, pb := prefer[list[a].f.Path], prefer[list[b].f.Path]; pa != pb {
			return pa
		}
		if len(list[a].words) != len(list[b].words) {
			return len(list[a].words) > len(list[b].words)
		}
		if len(list[a].lines) != len(list[b].lines) {
			return len(list[a].lines) > len(list[b].lines)
		}
		// The general implementation before a variant in a deeper package (service/ before service/variant/).
		if da, db := strings.Count(list[a].f.Path, "/"), strings.Count(list[b].f.Path, "/"); da != db {
			return da < db
		}
		return list[a].f.Path+fmt.Sprint(list[a].f.From) < list[b].f.Path+fmt.Sprint(list[b].f.From)
	})
	out := []Function{}
	for _, f := range list {
		if len(out) == n {
			break
		}
		src := files[f.f.Path]
		sort.Ints(f.lines)
		f.lines = uniqueInts(f.lines)
		shown := linesToShow(f.f.From, f.lines, span, len(src))
		numbered, prev := []string{}, 0
		if f.f.From > 0 && (len(shown) == 0 || shown[0] > f.f.From) {
			numbered = append(numbered, fmt.Sprintf("%4d│ %s", f.f.From, Redact(src[f.f.From-1])))
			prev = f.f.From
		}
		for _, k := range shown {
			if prev > 0 && k > prev+1 {
				numbered = append(numbered, "    │ …")
			}
			numbered = append(numbered, fmt.Sprintf("%4d│ %s", k, Redact(src[k-1])))
			prev = k
		}
		f.f.Code = strings.Join(numbered, "\n")
		if len(shown) > 0 {
			f.f.From, f.f.To = shown[0], shown[len(shown)-1]
		}
		in := map[int]bool{}
		for _, k := range shown {
			in[k] = true
		}
		for _, l := range f.lines {
			if !in[l] {
				f.f.Outside = append(f.f.Outside, l)
			}
		}
		if name := definedName(f.f.Name); name != "" {
			f.f.Callers = s.callers(r, name, f.f.Path, 4)
		}
		for w := range f.words {
			f.f.Words = append(f.f.Words, w)
		}
		sort.Strings(f.f.Words)
		out = append(out, f.f)
	}
	return out, shortRef(r.ref)
}

// CodeFile reports whether a path is source code rather than configuration or documentation.
func CodeFile(p string) bool { return codeFile(p) }

func codeFile(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".go", ".java", ".kt", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".py", ".cs", ".rb", ".php", ".scala", ".rs", ".swift", ".dart", ".sql":
		return true
	}
	return false
}

func wholeWord(text, name string) bool {
	for from := 0; ; {
		k := strings.Index(text[from:], name)
		if k < 0 {
			return false
		}
		a, b := from+k, from+k+len(name)
		word := func(c byte) bool {
			return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		}
		if (a == 0 || !word(text[a-1])) && (b == len(text) || !word(text[b])) {
			return true
		}
		from = a + 1
	}
}

// Excerpt returns up to span numbered lines of a cited file at its commit: the cited range, or the
// definition of the first of names when the link has none. Inside a longer range or definition the
// lines start just before the first line holding one of focus (the literal the claim is about), with
// the range's first line kept above. label names what was shown.
func (s *Sources) Excerpt(a Anchor, names, focus []string, span int) (string, string, bool) {
	r := s.repo(a.Repo)
	if r.path == "" || !s.files.commit(r.path, a.Commit) {
		return "", "", false
	}
	content, ok := s.files.file(r.path, a.Commit, a.Path)
	if !ok {
		return "", "", false
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	from, to, label := a.From, a.To, ""
	if from == 0 {
		_, line, name, found := s.Symbol(a, names, 1)
		if !found {
			return "", "", false
		}
		from, to, label = line, len(lines), " · "+name
		for k := line; k < len(lines); k++ { // to the next definition
			if isFuncStart(lines[k]) {
				to = k
				break
			}
		}
	}
	if to < from {
		to = from
	}
	if from > len(lines) {
		return "", "", false
	}
	to = min(to, len(lines))
	start := from
	if to-from+1 > span {
		for k := from; k <= to && start == from; k++ {
			for _, f := range focus {
				if f != "" && strings.Contains(lines[k-1], f) {
					if k-from >= span-2 {
						start = max(from+1, k-min(3, span/2))
					}
					break
				}
			}
		}
	}
	out := []string{}
	if start > from {
		out = append(out, fmt.Sprintf("%4d│ %s", from, Redact(lines[from-1])), "    │ …")
	}
	for k := start; k <= min(to, start+span-1); k++ {
		out = append(out, fmt.Sprintf("%4d│ %s", k, Redact(lines[k-1])))
	}
	return strings.Join(out, "\n"), fmt.Sprintf("%s#L%d%s", a.Path, from, label), true
}

// excluded is what a code read leaves out: tests and fixtures (at any depth, the root included),
// generated and minified assets, source maps and lock files.
var excluded = []string{":(exclude)*_test.go", ":(exclude)*.test.*", ":(exclude)*.spec.*", ":(exclude)*Test*.java",
	":(exclude)test/**", ":(exclude)tests/**", ":(exclude)**/test/**", ":(exclude)**/tests/**", ":(exclude)**/__tests__/**",
	":(exclude)*.svg", ":(exclude)*.map", ":(exclude)*.min.js", ":(exclude)*.min.css", ":(exclude)*.lock", ":(exclude)package-lock.json", ":(exclude)*.snap"}

// Redact hides the value of a credential in a line of code or configuration (a secret-named key's
// literal, a signed URL, a password in a connection string): kos never prints a secret it reads.
func Redact(line string) string {
	if m := lineKV.FindStringSubmatchIndex(line); m != nil {
		key, value := line[m[2]:m[3]], line[m[4]:m[5]]
		if credentialEntry(key, value) {
			return line[:m[4]] + "‹redacted›" + line[m[5]:]
		}
	}
	return credentialValue.ReplaceAllString(line, "‹redacted›")
}

// Tracked lists the repositories of the vault that have a local checkout.
func Tracked(vault string) []string {
	inputs, _ := discoverRepositories(vault, nil)
	out := []string{}
	for _, in := range inputs {
		if in.Path != "" {
			out = append(out, in.Name)
		}
	}
	sort.Strings(out)
	return out
}

// Resolve names a tracked repository with a local checkout: its name, or false with the reason.
func (s *Sources) Resolve(name string) (string, string, bool) {
	r := s.repo(name)
	if r.head == "" {
		return "", r.err, false
	}
	return name[strings.Index(name, "/")+1:], shortRef(r.ref), true
}

// Search greps a repository's reference branch for an extended regular expression: every matching
// line of code or configuration (tests left out unless tests), the enclosing function of each, and
// how many files were searched, so that no match is a statement with its scope.
func (s *Sources) Search(ownerRepo, pattern, glob string, ignoreCase, tests, word bool) ([]Hit, map[Hit]string, int, error) {
	r := s.repo(ownerRepo)
	if r.head == "" {
		return nil, nil, 0, fmt.Errorf("%s", r.err)
	}
	args := []string{"grep", "-n", "-I", "-E"}
	if ignoreCase {
		args = append(args, "-i")
	}
	if word {
		args = append(args, "-w")
	}
	args = append(args, "-e", pattern, r.head, "--")
	if glob != "" {
		args = append(args, ":(glob)"+glob)
	} else {
		args = append(args, ".")
	}
	if !tests {
		args = append(args, excluded...)
	}
	cmd := execGit(r.path, args...)
	b, e := cmd.Output()
	if e != nil {
		if ee, ok := e.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
			return nil, nil, 0, fmt.Errorf("git grep: %v", e)
		}
	}
	hits, in := []Hit{}, map[Hit]string{}
	files := map[string][]string{}
	for _, l := range strings.Split(string(b), "\n") {
		parts := strings.SplitN(l, ":", 4)
		if len(parts) != 4 {
			continue
		}
		n, e := strconv.Atoi(parts[2])
		if e != nil {
			continue
		}
		if len(parts[3]) > 500 {
			continue // a minified or generated line: nothing a reader can use
		}
		h := Hit{Path: parts[1], Line: n, Text: Redact(strings.TrimSpace(parts[3]))}
		hits = append(hits, h)
		if codeFile(h.Path) {
			src, ok := files[h.Path]
			if !ok {
				content, _ := s.files.file(r.path, r.head, h.Path)
				src = strings.Split(content, "\n")
				files[h.Path] = src
			}
			for k := min(n, len(src)) - 1; k >= 0; k-- {
				if isFuncStart(src[k]) {
					in[h] = definedName(src[k])
					break
				}
			}
		}
	}
	list, _ := gitOutput(r.path, "ls-tree", "-r", "--name-only", r.head)
	return hits, in, strings.Count(list, "\n") + 1, nil
}

// File returns a file of the reference branch as numbered lines from..to (0 means to the end).
func (s *Sources) File(ownerRepo, path string, from, to int) (string, int, error) {
	r := s.repo(ownerRepo)
	if r.head == "" {
		return "", 0, fmt.Errorf("%s", r.err)
	}
	content, ok := s.files.file(r.path, r.head, path)
	if !ok {
		return "", 0, fmt.Errorf("%s is not in %s at %s", path, ownerRepo, shortRef(r.ref))
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	from = max(from, 1)
	if to == 0 || to > len(lines) {
		to = len(lines)
	}
	out := []string{}
	for k := from; k <= to; k++ {
		out = append(out, fmt.Sprintf("%4d│ %s", k, Redact(lines[k-1])))
	}
	return strings.Join(out, "\n"), len(lines), nil
}

// Definition finds where name is defined in the code of the reference branch and returns the whole
// definition (up to span lines, numbered), its path and line, and its callers with the lines after
// each call: what the caller does with the result.
func (s *Sources) Definition(ownerRepo, name, glob string, span int) ([]Function, error) {
	r := s.repo(ownerRepo)
	if r.head == "" {
		return nil, fmt.Errorf("%s", r.err)
	}
	text, _ := gitOutput(r.path, append([]string{"grep", "-n", "-I", "-w", "-F", "-e", name, r.head, "--", "."}, excluded...)...)
	out := []Function{}
	for _, l := range strings.Split(text, "\n") {
		parts := strings.SplitN(l, ":", 4)
		if len(parts) != 4 || !codeFile(parts[1]) || definedName(parts[3]) != name {
			continue
		}
		if glob != "" {
			if ok, _ := filepath.Match(glob, parts[1]); !ok && !strings.Contains(parts[1], strings.Trim(glob, "*")) {
				continue
			}
		}
		line, _ := strconv.Atoi(parts[2])
		content, _ := s.files.file(r.path, r.head, parts[1])
		src := strings.Split(strings.TrimRight(content, "\n"), "\n")
		end := len(src)
		for k := line; k < len(src); k++ {
			if isFuncStart(src[k]) {
				end = k
				break
			}
		}
		for end > line && strings.TrimSpace(src[end-1]) == "" {
			end--
		}
		to := min(end, line+span-1)
		numbered := []string{}
		for k := line; k <= to; k++ {
			numbered = append(numbered, fmt.Sprintf("%4d│ %s", k, Redact(src[k-1])))
		}
		if to < end {
			numbered = append(numbered, fmt.Sprintf("    │ … %d more lines to L%d", end-to, end))
		}
		f := Function{Path: parts[1], Name: strings.TrimSpace(parts[3]), From: line, To: end, Code: strings.Join(numbered, "\n")}
		f.Callers = s.callers(r, name, parts[1], 6)
		out = append(out, f)
	}
	return out, nil
}

// Callers lists where a function is called or passed, with the function each call is in.
func (s *Sources) Callers(ownerRepo, name, defFile string, n int) []Hit {
	r := s.repo(ownerRepo)
	if r.head == "" {
		return nil
	}
	return s.callers(r, name, defFile, n)
}

// CallContext returns the call line and the lines after it, numbered: how a caller handles the result.
func (s *Sources) CallContext(ownerRepo string, h Hit, after int) string {
	r := s.repo(ownerRepo)
	content, ok := s.files.file(r.path, r.head, h.Path)
	if !ok {
		return ""
	}
	src := strings.Split(content, "\n")
	out := []string{}
	for k := h.Line; k <= min(h.Line+after, len(src)); k++ {
		out = append(out, fmt.Sprintf("%4d│ %s", k, Redact(src[k-1])))
	}
	return strings.Join(out, "\n")
}

// Has reports whether the cited file exists at its commit in the local checkout.
func (s *Sources) Has(a Anchor) bool {
	r := s.repo(a.Repo)
	if r.path == "" || !s.files.commit(r.path, a.Commit) {
		return false
	}
	_, ok := s.files.file(r.path, a.Commit, a.Path)
	return ok
}
