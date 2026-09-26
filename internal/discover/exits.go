package discover

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Exit is a line where a function leaves its main path: it throws, exits the process, responds
// with an error status, rejects, nacks or acks a message, returns an error, or catches an error
// and carries on. Names are what a note would write to describe that path.
type Exit struct {
	Line  int
	Kind  string
	Text  string
	When  string   // the condition of the block the exit is in: if (…) {, } catch (e) {
	At    int      // the line of When
	Names []string // identifiers, messages and status codes of the path; empty when it is generic
}

// Propagates reports whether the exit only passes on an error its callee returned.
func (e Exit) Propagates() bool { return e.Kind == "propagates" }

var (
	literalRe   = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|` + "`[^`]*`")
	exitProcess = regexp.MustCompile(`\b(?:process\.exit|os\.Exit|sys\.exit|System\.exit|log\.Fatal\w*|exit)\s*\(`)
	throwRe     = regexp.MustCompile(`\b(?:throw|raise)\b`)
	panicRe     = regexp.MustCompile(`\bpanic\(`)
	rejectRe    = regexp.MustCompile(`\breject\(|Promise\.reject\(`)
	nackRe      = regexp.MustCompile(`\.(?:nack|Nack|nak|Nak)\(`)
	ackRe       = regexp.MustCompile(`\.(?:ack|Ack)\(`)
	statusRe    = regexp.MustCompile(`(?:\.status|sendStatus|WriteHeader|status_code\s*=|StatusCode:?\s*=?|\bstatus:)\s*\(?\s*([1-5]\d\d)\b`)
	httpStatus  = regexp.MustCompile(`\bHttpStatus\.([A-Z_]+)|\bhttp\.Status(\w+)|\bStatus(BadRequest|Conflict|NotFound|Unauthorized|Forbidden|InternalServerError|UnprocessableEntity|ServiceUnavailable)\b`)
	returnErr   = regexp.MustCompile(`^\s*return\b(.*)$`)
	errValue    = regexp.MustCompile(`\b(?:errors\.New|fmt\.Errorf|status\.Errorf?|Err[A-Z]\w*|\w+Error\{|&\w+Error\{)`)
	catchRe     = regexp.MustCompile(`\bcatch\b\s*(?:\(|\{)|\.catch\(|^\s*except\b|\bif\s+err\s*!=\s*nil\s*\{|\brecover\(\)`)
	loopJump    = regexp.MustCompile(`^\s*(continue|break)\b\s*;?\s*$`)
	calledName  = regexp.MustCompile(`\.?([A-Za-z_]\w{4,})\(`)
	identRe     = regexp.MustCompile(`[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*`)
)

// okStatus are the HttpStatus names of a success response: not an exit.
var okStatus = map[string]bool{"OK": true, "CREATED": true, "ACCEPTED": true, "NO_CONTENT": true, "Ok": true, "OK_": true, "Created": true, "Accepted": true, "NoContent": true}

// genericName are words that every error path writes: they do not say which path it is.
var genericName = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`Error Errorf Exception TypeError RangeError RuntimeException IllegalStateException
		IllegalArgumentException errors New new throw raise return process exit reject Promise message msg nack ack
		logger console log res status json send err error e ex http HttpStatus StatusCode fmt panic this self await
		Status Fatal Fatalf sys System HttpException Response JSONResponse ResponseEntity context ctx`) {
		genericName[strings.ToLower(w)] = true
	}
}

// functionEnd returns the last line of the function that starts at line start (1-based): the line
// that closes its braces, the last line indented under a Python def, or the line before the next
// function when neither is recognizable.
func functionEnd(src []string, start int, path string) int {
	if start < 1 || start > len(src) {
		return start
	}
	limit := min(len(src), start+800)
	if strings.HasSuffix(path, ".py") {
		indent := leading(src[start-1])
		end := start
		for k := start + 1; k <= limit; k++ {
			if strings.TrimSpace(src[k-1]) == "" {
				continue
			}
			if leading(src[k-1]) <= indent && !strings.HasPrefix(strings.TrimSpace(src[k-1]), ")") {
				break
			}
			end = k
		}
		return end
	}
	if end, ok := blockEnd(src, start, 0, limit); ok {
		return end
	}
	end := len(src)
	for k := start; k < len(src); k++ {
		if isFuncStart(src[k]) {
			end = k
			break
		}
	}
	for end > start && strings.TrimSpace(src[end-1]) == "" {
		end--
	}
	return end
}

// blockEnd follows the braces from column col of line start and returns the line that closes the
// first one opened, or false when none opens within the first lines.
func blockEnd(src []string, start, col, limit int) (int, bool) {
	depth, opened := 0, false
	for k := start; k <= limit; k++ {
		line := codeOnly(src[k-1])
		if k == start {
			line = codeOnly(src[k-1][min(col, len(src[k-1])):])
		}
		for _, c := range line {
			switch c {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
			}
			if opened && depth <= 0 {
				return k, true
			}
		}
		if !opened && k-start >= 6 {
			return 0, false
		}
	}
	return 0, false
}

// codeOnly drops a line's string literals and trailing comment, so their braces do not count.
func codeOnly(line string) string {
	line = literalRe.ReplaceAllString(line, `""`)
	if k := strings.Index(line, "//"); k >= 0 {
		line = line[:k]
	}
	return line
}

func leading(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func comment(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*")
}

// exitsIn lists the exits of the function on lines from..to (1-based, inclusive): the lines that
// leave it, each with the condition of the block it is in. Returns inside nested functions
// (callbacks) are theirs, not this function's.
func exitsIn(src []string, from, to int, path string) []Exit {
	src = maskRawStrings(src, path)
	nested := nestedFunctions(src, from, to, path)
	// The line that opens the body: a signature on several lines ends there, and a block opened
	// there is the function itself, not a condition.
	body := from
	for k := from; k <= min(to, len(src)) && !strings.HasSuffix(path, ".py"); k++ {
		if strings.Contains(codeOnly(src[k-1]), "{") {
			body = k
			break
		}
	}
	out := []Exit{}
	for k := max(from, 1); k <= min(to, len(src)); k++ {
		line := src[k-1]
		if comment(line) {
			continue
		}
		code := codeOnly(line)
		text := Redact(strings.TrimSpace(line))
		kind := ""
		switch {
		case exitProcess.MatchString(code):
			kind = "exits the process"
		case panicRe.MatchString(code):
			kind = "panics"
		case throwRe.MatchString(code):
			kind = "throws"
			if t := strings.TrimSpace(strings.TrimSuffix(throwRe.ReplaceAllString(code, ""), ";")); t == "" || t == "e" || t == "err" || t == "error" || t == "ex" {
				kind = "propagates" // a rethrow: the path is whoever threw it
			}
		case rejectRe.MatchString(code):
			kind = "rejects"
		case nackRe.MatchString(code):
			kind = "nacks"
		case ackRe.MatchString(code):
			kind = "acks"
		case catchRe.MatchString(code):
			if continues(src, k, path) {
				kind = "catches and continues"
				if strings.Contains(code, "err != nil") {
					kind = "continues after the error"
				}
			}
		case loopJump.MatchString(code) && !nested[k]:
			// In a retry loop these are the paths: try again, give up.
			if word := loopJump.FindStringSubmatch(code)[1]; word == "continue" {
				kind = "goes to the next iteration"
			} else if inLoop(src, body, k, path) {
				kind = "leaves the loop"
			}
		}
		if kind == "" {
			if m := statusRe.FindStringSubmatch(code); m != nil && m[1][0] >= '4' {
				kind = "responds " + m[1]
			} else if m := httpStatus.FindStringSubmatch(code); m != nil && !okStatus[m[1]+m[2]+m[3]] {
				kind = "responds " + m[1] + m[2] + m[3]
			} else if m := returnErr.FindStringSubmatch(code); m != nil && !nested[k] {
				value := strings.TrimSuffix(strings.TrimSpace(m[1]), ";")
				if raw := returnErr.FindStringSubmatch(withoutComment(line)); raw != nil {
					value = strings.TrimSuffix(strings.TrimSpace(raw[1]), ";") // its messages kept: fmt.Sprintf("%s-%s", …)
				}
				last := strings.TrimSpace(lastValue(value))
				switch {
				case last == "err" || last == "e":
					kind = "propagates"
				case errValue.MatchString(last):
					kind = "returns an error"
				case value == "":
					kind = "returns"
				default:
					kind = "returns " + trimValue(value)
				}
			}
		}
		if kind == "" {
			continue
		}
		e := Exit{Line: k, Kind: kind, Text: text}
		if k > body {
			e.When, e.At = enclosing(src, body, k, path)
		}
		if kind != "propagates" {
			e.Names = exitNames(line, kind)
			if e.When != "" {
				e.Names = append(e.Names, exitNames(e.When, "")...)
				// What the lines before it log and call name the path too: 'Error parse data', rollback(…).
				for j := max(e.At+1, k-12); j < k; j++ {
					e.Names = append(e.Names, literals(src[j-1])...)
					for _, m := range calledName.FindAllStringSubmatch(codeOnly(src[j-1]), -1) {
						e.Names = append(e.Names, m[1])
					}
				}
			}
			if kind == "catches and continues" {
				// What the handler writes (its log message) names the path better than catch (e).
				end, _ := blockEnd(src, k, max(catchRe.FindStringIndex(line)[0], 0), min(len(src), k+12))
				for j := k + 1; j <= end && j <= len(src); j++ {
					e.Names = append(e.Names, exitNames(src[j-1], "")...)
				}
			}
			e.Names = uniqueStrings(e.Names)
		}
		// msg.Nack(); return: one path, not two.
		if e.Kind == "returns" && len(out) > 0 && out[len(out)-1].At == e.At && e.Line-out[len(out)-1].Line <= 2 {
			continue
		}
		out = append(out, e)
	}
	// Falling off the end is an exit too: after a retry loop, often the give-up path.
	last := min(to, len(src)) - 1
	for last > body && (strings.Trim(src[last-1], " \t});") == "" || comment(src[last-1])) {
		last-- // blank lines, comments and the braces closing try, catch or if
	}
	if last > body && !strings.HasSuffix(path, ".py") {
		t := strings.TrimSpace(codeOnly(src[last-1]))
		if !returnErr.MatchString(src[last-1]) && !throwRe.MatchString(t) && !exitProcess.MatchString(t) && !panicRe.MatchString(t) {
			e := Exit{Line: min(to, len(src)), Kind: "reaches its end", Text: strings.TrimSpace(src[min(to, len(src))-1])}
			for j := max(body+1, last-2); j <= last; j++ {
				e.Names = append(e.Names, literals(src[j-1])...)
			}
			if len(e.Names) > 0 {
				e.Kind += " after '" + e.Names[len(e.Names)-1] + "'"
			}
			out = append(out, e)
		}
	}
	return out
}

func trimValue(v string) string {
	if r := []rune(v); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return v
}

var nestedOpen = regexp.MustCompile(`=>\s*\{|\bfunction\b[^(]*\(|\bfunc\s*\(`)

// nestedFunctions marks the lines of the callbacks and closures defined inside from..to.
func nestedFunctions(src []string, from, to int, path string) map[int]bool {
	out := map[int]bool{}
	if strings.HasSuffix(path, ".py") {
		return out
	}
	for k := from + 1; k <= min(to, len(src)); k++ {
		code := codeOnly(src[k-1])
		loc := nestedOpen.FindStringIndex(code)
		if loc == nil {
			continue
		}
		end, ok := blockEnd(src, k, loc[0], min(to, len(src)))
		if !ok {
			continue
		}
		for j := k + 1; j <= end; j++ {
			out[j] = true
		}
	}
	return out
}

// enclosing is the opening line of the innermost block around line k, below the function's own
// first line, or "" when the exit is at the function's top level.
func enclosing(src []string, from, k int, path string) (string, int) {
	if strings.HasSuffix(path, ".py") {
		indent := leading(src[k-1])
		for j := k - 1; j > from; j-- {
			if t := strings.TrimSpace(src[j-1]); t != "" && leading(src[j-1]) < indent {
				return t, j
			}
		}
		return "", 0
	}
	depth := 0
	for j := k - 1; j > from; j-- {
		code := codeOnly(src[j-1])
		for i := len(code) - 1; i >= 0; i-- {
			switch code[i] {
			case '}':
				depth++
			case '{':
				depth--
			}
			if depth < 0 {
				return strings.TrimSpace(src[j-1]), j
			}
		}
	}
	return "", 0
}

// continues reports whether the error handler opened at line k ends without leaving: no throw,
// return, exit, reject or nack in its body.
func continues(src []string, k int, path string) bool {
	line := src[k-1]
	var end int
	if strings.HasSuffix(path, ".py") {
		end = k
		indent := leading(line)
		for j := k + 1; j <= min(len(src), k+20); j++ {
			if strings.TrimSpace(src[j-1]) == "" {
				continue
			}
			if leading(src[j-1]) <= indent {
				break
			}
			end = j
		}
	} else {
		loc := catchRe.FindStringIndex(line)
		e, ok := blockEnd(src, k, loc[0], min(len(src), k+20))
		if !ok {
			return false
		}
		end = e
	}
	if end == k {
		// catch {} on one line
		body := line[catchRe.FindStringIndex(line)[1]:]
		return !leaves(codeOnly(body))
	}
	for j := k + 1; j <= end; j++ {
		if !comment(src[j-1]) && leaves(codeOnly(src[j-1])) {
			return false
		}
	}
	return true
}

var leavesRe = regexp.MustCompile(`\b(?:return|throw|raise|panic|reject|continue|break)\b|\.(?:nack|Nack|nak)\(`)

func leaves(code string) bool {
	return leavesRe.MatchString(code) || exitProcess.MatchString(code)
}

// exitNames are the words of an exit a note would write: its identifiers with a capital or an
// underscore, its messages and its status code.
func exitNames(line, kind string) []string {
	out := literals(line)
	for _, id := range identRe.FindAllString(literalRe.ReplaceAllString(line, " "), -1) {
		for _, part := range strings.Split(id, ".") {
			if len(part) < 5 || genericName[strings.ToLower(part)] || !strings.ContainsAny(part, "ABCDEFGHIJKLMNOPQRSTUVWXYZ_") {
				continue
			}
			out = append(out, part)
		}
	}
	switch {
	case strings.HasPrefix(kind, "responds "):
		out = append(out, strings.TrimPrefix(kind, "responds "))
	case kind == "exits the process":
		out = append(out, "exit")
	case kind == "nacks":
		out = append(out, "nack")
	case kind == "acks":
		out = append(out, "ack")
	case kind == "panics":
		out = append(out, "panic")
	}
	return uniqueStrings(out)
}

// ProcessExits lists where the code of the reference branch ends its own process (process.exit,
// os.Exit, log.Fatal, sys.exit), tests left out, each with the function it is in: paths that
// drop in-flight work, which notes often leave out.
func (s *Sources) ProcessExits(ownerRepo string) ([]Hit, string) {
	r := s.repo(ownerRepo)
	if r.head == "" {
		return nil, ""
	}
	text, _ := gitOutput(r.path, append([]string{"grep", "-n", "-I", "-E", `(process\.exit|os\.Exit|sys\.exit|System\.exit|log\.Fatal)`, r.head, "--", "."}, excluded...)...)
	out := []Hit{}
	for _, l := range strings.Split(text, "\n") {
		parts := strings.SplitN(l, ":", 4)
		if len(parts) != 4 || !codeFile(parts[1]) || comment(parts[3]) || !exitProcess.MatchString(codeOnly(parts[3])) {
			continue
		}
		line, _ := strconv.Atoi(parts[2])
		h := Hit{Path: parts[1], Line: line, Text: Redact(strings.TrimSpace(parts[3]))}
		if content, ok := s.files.file(r.path, r.head, h.Path); ok {
			src := strings.Split(content, "\n")
			for k := min(line, len(src)) - 1; k >= 1; k-- {
				if isFuncStart(src[k-1]) {
					h.In = definedName(src[k-1])
					break
				}
			}
		}
		out = append(out, h)
	}
	return out, shortRef(r.ref)
}

// ExitCall is the call that ends the process on a line: process.exit(1), os.Exit(2).
func ExitCall(line string) string {
	loc := exitProcess.FindStringIndex(line)
	if loc == nil {
		return ""
	}
	return line[loc[0]:loc[1]]
}

// literals are the messages a line writes in quotes, format verbs and interpolations left out.
func literals(line string) []string {
	out := []string{}
	for _, m := range literalRe.FindAllString(line, -1) {
		s := strings.TrimSpace(strings.Trim(m, "\"'`"))
		s = strings.NewReplacer(`\n`, " ", `\t`, " ", `\"`, `"`).Replace(s)
		s = regexp.MustCompile(`%[-+ #0-9.]*[a-zA-Z]|\$\{[^}]*\}|\{[^}]*\}`).ReplaceAllString(s, " ")
		s = strings.Trim(strings.Join(strings.Fields(s), " "), " :.,-")
		if len([]rune(s)) >= 5 && strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			out = append(out, s)
		}
	}
	return out
}

// lastValue is the last of the comma-separated values a return writes, commas inside calls kept.
func lastValue(v string) string {
	depth, cut := 0, 0
	for i, c := range v {
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				cut = i + 1
			}
		}
	}
	return v[cut:]
}

// Callee is a function of the same repository that a function calls, at its first call.
type Callee struct {
	Name string
	Line int        // the first call's line in the caller
	Defs []Function // its definitions, each with its exits
}

// callWord are names that are called everywhere and defined nowhere in a repository worth reading.
var callWord = map[string]bool{"log": true, "info": true, "warn": true, "error": true, "debug": true, "push": true, "map": true, "filter": true, "forEach": true, "then": true, "catch": true, "resolve": true, "reject": true, "String": true, "Number": true, "Boolean": true, "Date": true, "toISOString": true, "stringify": true, "parse": true, "append": true, "make": true, "len": true, "print": true, "printf": true, "Errorf": true, "Sprintf": true, "Println": true, "equals": true, "get": true, "set": true, "has": true, "format": true, "require": true, "super": true, "constructor": true, "func": true, "function": true}

// Callees lists the functions of the repository that f calls (its body at the reference branch),
// in call order, each with its definitions; at most n.
func (s *Sources) Callees(ownerRepo string, f Function, n int) []Callee {
	r := s.repo(ownerRepo)
	if r.head == "" || f.Start == 0 {
		return nil
	}
	content, ok := s.files.file(r.path, r.head, f.Path)
	if !ok {
		return nil
	}
	src := maskRawStrings(strings.Split(content, "\n"), f.Path)
	own := definedName(f.Name)
	// Its own name is skipped only when called bare (recursion): s.useCase.Handle( from a Handle
	// reaches another type's method, often the one that matters.
	seen := map[string]bool{}
	out := []Callee{}
	for k := f.Start + 1; k <= min(f.End, len(src)) && len(out) < n; k++ {
		if comment(src[k-1]) {
			continue
		}
		for _, m := range calleeName.FindAllStringSubmatch(codeOnly(src[k-1]), -1) {
			name := m[1]
			if seen[name] || callWord[name] || controlWord[name] || len(name) < 3 {
				continue
			}
			if name == own && !strings.Contains(src[k-1], "."+name+"(") {
				continue
			}
			seen[name] = true
			defs, _ := s.definition(ownerRepo, name, "", 150, false)
			if name == own {
				kept := defs[:0]
				for _, d := range defs {
					if !(d.Path == f.Path && d.Start == f.Start) {
						kept = append(kept, d)
					}
				}
				defs = kept
			}
			if len(defs) == 0 {
				continue
			}
			if strings.Contains(src[k-1], "this."+name+"(") || strings.Contains(src[k-1], "self."+name+"(") {
				// A method of the same object: the definition in this file, when there is one.
				own := []Function{}
				for _, d := range defs {
					if d.Path == f.Path {
						own = append(own, d)
					}
				}
				if len(own) > 0 {
					defs = own
				}
			}
			if len(defs) > 2 {
				defs = akin(defs, f.Path)
			}
			out = append(out, Callee{Name: name, Line: k, Defs: defs})
			if len(out) == n {
				break
			}
		}
	}
	return out
}

var calleeName = regexp.MustCompile(`(?:^|[^\w$])([A-Za-z_$][\w$]*)\s*\(`)

var (
	loopOpener   = regexp.MustCompile(`^\s*(?:\}\s*)?(?:for|while|do)\b|\.forEach\(`)
	switchOpener = regexp.MustCompile(`^\s*(?:\}\s*)?(?:switch|select|case|default)\b`)
)

// inLoop reports whether line k sits in a loop before any switch: a break there leaves the loop.
func inLoop(src []string, body, k int, path string) bool {
	for at := k; at > body; {
		when, line := enclosing(src, body, at, path)
		if line == 0 {
			return false
		}
		switch {
		case switchOpener.MatchString(when):
			return false
		case loopOpener.MatchString(when):
			return true
		}
		at = line
	}
	return false
}

// akin keeps, of several definitions sharing a name (an interface method), those whose file names
// the caller's package: transactionconfirmed/subscriber.go calling Handle reaches
// service/transaction_confirmed.go, not service/giftcard_activation.go.
func akin(defs []Function, caller string) []Function {
	pkg := normName(filepath.Base(filepath.Dir(caller)))
	if len(pkg) < 6 {
		return defs
	}
	kept := []Function{}
	for _, d := range defs {
		// The file name or a directory of the definition names the caller's package:
		// service/transaction_confirmed.go, saletransactionsconfirmed/service/service.go.
		parts := strings.Split(filepath.ToSlash(strings.TrimSuffix(d.Path, filepath.Ext(d.Path))), "/")
		for _, part := range parts {
			base := normName(part)
			if len(base) >= 6 && (strings.Contains(base, pkg) || strings.Contains(pkg, base)) {
				kept = append(kept, d)
				break
			}
		}
	}
	if len(kept) == 0 {
		return defs
	}
	return kept
}

func normName(s string) string {
	return strings.Map(func(c rune) rune {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			return c
		case c >= 'A' && c <= 'Z':
			return c + 'a' - 'A'
		}
		return -1
	}, s)
}

// maskRawStrings blanks the lines inside a multi-line backtick literal (a Go raw string, a JS
// template): SQL or text there is not code, and EXTRACT( in a query is no call.
func maskRawStrings(src []string, path string) []string {
	switch filepath.Ext(path) {
	case ".go", ".js", ".ts", ".tsx", ".jsx", ".mjs", ".cjs":
	default:
		return src
	}
	out, open := make([]string, len(src)), false
	for k, l := range src {
		n := strings.Count(literalRe.ReplaceAllStringFunc(l, func(q string) string {
			if strings.HasPrefix(q, "`") {
				return q
			}
			return `""`
		}), "`")
		switch {
		case open && n%2 == 1:
			out[k], open = "`"+l[strings.LastIndex(l, "`")+1:], false // the literal closes here
		case open:
			out[k] = ""
		default:
			out[k] = l
			if n%2 == 1 {
				open = true
			}
		}
	}
	return out
}

// withoutComment drops a trailing // comment outside quotes.
func withoutComment(line string) string {
	code := codeOnly(line)
	if k := strings.Index(code, "//"); k < 0 && strings.Contains(line, "//") {
		if j := strings.LastIndex(line, "//"); j >= 0 && !strings.Contains(line[j:], `"`) && !strings.Contains(line[j:], "'") {
			return line[:j]
		}
	}
	return line
}

var (
	classDecl  = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:public\s+|abstract\s+|final\s+|sealed\s+|data\s+|open\s+)*(?:class|struct|object)\s+([A-Z]\w*)`)
	goReceiver = regexp.MustCompile(`^\s*func\s*\(\s*\w*\s*\*?\s*([A-Z]?\w+)(?:\[[^\]]*\])?\s*\)`)
)

// Registrations: for a method nothing calls by name (a framework does: an interceptor, a handler, a
// listener), the class or receiver type it belongs to and the lines outside its own file that use
// that type: where it is provided, registered or constructed.
func (s *Sources) Registrations(ownerRepo string, f Function, n int) (string, []Hit) {
	if f.src == nil || f.Start == 0 {
		return "", nil
	}
	class := ""
	if m := goReceiver.FindStringSubmatch(f.src[f.Start-1]); m != nil {
		class = m[1]
	}
	for k := f.Start - 1; k >= 1 && class == ""; k-- {
		if m := classDecl.FindStringSubmatch(f.src[k-1]); m != nil && leading(f.src[k-1]) < leading(f.src[f.Start-1]) {
			class = m[1]
		}
	}
	if class == "" {
		return "", nil
	}
	pattern := regexp.QuoteMeta(class)
	if strings.HasSuffix(f.Path, ".go") {
		pattern += "|New" + regexp.QuoteMeta(class) // Go constructs through NewT more often than T{}
	}
	hits, _, _, err := s.Search(ownerRepo, pattern, "", false, false, true)
	if err != nil {
		return class, nil
	}
	out := []Hit{}
	for _, h := range hits {
		t := strings.TrimSpace(h.Text)
		if h.Path == f.Path || !codeFile(h.Path) || comment(t) || importLine.MatchString(t) || classDecl.MatchString(t) || interfaceMethod.MatchString(t) {
			continue
		}
		out = append(out, h)
		if len(out) == n {
			break
		}
	}
	return class, out
}
