package discover

import (
	"fmt"
	"regexp"
	"strings"
)

// Hazard is a path a redelivery can take after a failure part way through a function: an early
// success exit guarded by a check (a duplicate lookup) comes first, a write happens, then a failure
// hands the message back; on the retry the check can find what the first write left and leave
// before the writes that never happened.
type Hazard struct {
	Check, Early, Write, Fail int    // lines
	CheckCall, WriteCall      string // FindByID, Save
	Skipped                   []string
	EarlyKind, FailKind       string
}

func (h Hazard) String() string {
	s := fmt.Sprintf("a failure at L%d (%s) after the write at L%d (%s) hands the message back; on the retry the check at L%d (%s) can find what L%d wrote and leave at L%d (%s)", h.Fail, h.FailKind, h.Write, h.WriteCall, h.Check, h.CheckCall, h.Write, h.Early, h.EarlyKind)
	if len(h.Skipped) > 0 {
		s += " without " + strings.Join(h.Skipped, ", ")
	}
	return s
}

var (
	// A lookup goes through a repository, a client or a guard (repo.FindByID, this.guard.check): a
	// local helper (checkIfHealthcheck(flow)) inspects the message, not what an earlier try wrote.
	checkCall = regexp.MustCompile(`\.((?:[Ff]ind|[Gg]et|[Ee]xists?|[Cc]heck|[Ll]ookup|[Hh]as|[Rr]eserve|[Rr]egister|[Ii]sDuplicate|[Ss]een)\w*)\s*\(`)
	writeCall = regexp.MustCompile(`\b((?:[Ss]ave|[Cc]reate|[Ii]nsert|[Pp]ut|[Uu]psert|[Ss]tore|[Pp]ersist|[Ww]rite|[Pp]ublish|[Nn]otify|[Ss]end|[Uu]pdate|[Dd]elete|[Cc]ommit)\w*)\s*\(`)
	logCall   = regexp.MustCompile(`(?i)\b(log|logger|console|fmt\.Print)`)
)

// Hazards finds, in a function, the retry paths that skip work after a partial failure.
func (f Function) Hazards() []Hazard {
	if f.src == nil || len(f.Exits) == 0 {
		return nil
	}
	src := maskRawStrings(f.src, f.Path)
	type call struct {
		line int
		name string
	}
	calls := func(re *regexp.Regexp) []call {
		out := []call{}
		for k := f.Start + 1; k <= min(f.End, len(src)); k++ {
			l := src[k-1]
			if comment(l) || logCall.MatchString(l) {
				continue
			}
			if m := re.FindStringSubmatch(codeOnly(l)); m != nil {
				out = append(out, call{k, m[1]})
			}
		}
		return out
	}
	checks, writes := calls(checkCall), calls(writeCall)
	success := func(e Exit) bool {
		return e.Kind == "returns nil" || e.Kind == "returns true" || e.Kind == "returns" || e.Kind == "acks" || e.Kind == "returns null" || e.Kind == "returns None"
	}
	failure := func(e Exit) bool {
		switch {
		case e.Kind == "returns an error", e.Kind == "propagates", e.Kind == "throws", e.Kind == "nacks", e.Kind == "returns false", e.Kind == "rejects", e.Kind == "panics":
			return true
		case strings.HasPrefix(e.Kind, "responds 5"):
			return true
		}
		return false
	}
	out := []Hazard{}
	for _, early := range f.Exits {
		if !success(early) || early.At == 0 {
			continue
		}
		// The check guarding it: a lookup on its condition line or just above it.
		var ck *call
		for i := range checks {
			if c := checks[i]; c.line <= early.At && early.At-c.line <= 6 {
				ck = &checks[i]
			}
		}
		if ck == nil {
			continue
		}
		// A first write after the check, a later write, and a failure once the later one ran: the
		// first write is done, the later one is not, and the retry stops at the check.
		for wi, w := range writes {
			if w.line <= early.Line || wi+1 >= len(writes) {
				continue
			}
			w2 := writes[wi+1]
			for _, fl := range f.Exits {
				if !failure(fl) || fl.Line <= w2.line {
					continue
				}
				h := Hazard{Check: ck.line, CheckCall: ck.name, Early: early.Line, EarlyKind: early.Kind, Write: w.line, WriteCall: w.name, Fail: fl.Line, FailKind: fl.Kind}
				for _, later := range writes[wi+1:] {
					h.Skipped = append(h.Skipped, fmt.Sprintf("L%d %s", later.line, later.name))
				}
				h.Skipped = firstStrings(h.Skipped, 4)
				out = append(out, h)
				break
			}
			break
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

func firstStrings(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string{}, xs[:n]...), fmt.Sprintf("+%d more", len(xs)-n))
}
