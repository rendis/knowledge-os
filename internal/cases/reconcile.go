package cases

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"knowledge-os/internal/devhandoff"
)

// Reconciliation imports what a handoff worktree produced since the last mark into its development
// case, with a fixed mapping so that two runs over the same worktree write the same records:
//
//	commits since the mark      → evidence (source: the commits)
//	finding, verification,
//	definition, deviation       → evidence (source: the delta's evidence)
//	decision                    → decision
//	question                    → question
//	a delta without evidence    → question asking for it
//
// Definition changes and deviations are listed for review: which requirement they supersede is a
// judgment the agent records afterwards with add --supersedes.

var reconcileMark = regexp.MustCompile(`(?:reconciliado hasta|reconciled through) (\S+) / (DELTA-\d+|—)`)

func deltaNumber(id string) int {
	_, n, _ := strings.Cut(id, "-")
	v, _ := strconv.Atoi(n)
	return v
}

// lastMark returns the commit and delta number a handoff record was last reconciled through.
func lastMark(text, dh string) (string, int) {
	for _, l := range strings.Split(visible(text), "\n") {
		if m := recordDef.FindStringSubmatch(l); m != nil && m[1] == dh {
			all := reconcileMark.FindAllStringSubmatch(l, -1)
			if len(all) == 0 {
				return "", 0
			}
			last := all[len(all)-1]
			if last[2] == "—" {
				return last[1], 0
			}
			return last[1], deltaNumber(last[2])
		}
	}
	return "", 0
}

func reconcile(o options, out io.Writer) error {
	if o.worktree == "" {
		return errors.New("--worktree is required")
	}
	pkgs, commits := devhandoff.Tasks(o.worktree)
	if len(pkgs) == 0 {
		return fmt.Errorf("%s holds no handoff tasks", o.worktree)
	}
	caseIDs := map[string]bool{}
	for _, p := range pkgs {
		caseIDs[p.Case] = true
	}
	if o.id == "" {
		if len(caseIDs) != 1 {
			ids := []string{}
			for id := range caseIDs {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			return fmt.Errorf("the worktree holds tasks of several cases (%s): pass --id", strings.Join(ids, ", "))
		}
		for id := range caseIDs {
			o.id = id
		}
	}
	head, _ := exec.Command("git", "-C", o.worktree, "rev-parse", "--short", "HEAD").Output()
	headSHA := strings.TrimSpace(string(head))
	deltas := devhandoff.Deltas(o.worktree)
	o.dryRun = !o.apply
	return mutate(o, out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
		L := labels[loc]
		records, review, logParts := []string{}, []map[string]string{}, []string{}
		through := map[string]string{}
		for _, p := range pkgs {
			if p.Case != c.ID {
				continue
			}
			if !definedRecords(text)[p.Handoff] {
				return "", nil, nil, fmt.Errorf("%s is not recorded in the case: add --kind handoff --package handoffs/%s.md first", p.Handoff, p.Handoff)
			}
			markSHA, markDelta := lastMark(text, p.Handoff)
			added := func(r options) error {
				r.vault = o.vault
				next, id, _, e := buildRecord(r, c, text, loc, nil)
				if e != nil {
					return e
				}
				text = next
				records = append(records, id)
				return nil
			}
			fresh := []string{}
			for _, cm := range commits[p.Handoff] { // newest first
				sha, _, _ := strings.Cut(cm, " ")
				if markSHA != "" && (strings.HasPrefix(sha, markSHA) || strings.HasPrefix(markSHA, sha)) {
					break
				}
				fresh = append([]string{cm}, fresh...)
			}
			if len(fresh) > 0 {
				shas := []string{}
				for _, cm := range fresh {
					sha, _, _ := strings.Cut(cm, " ")
					shas = append(shas, sha)
				}
				if e := added(options{recKind: "evidence", level: "demonstrated", source: "commits " + strings.Join(shas, ", "),
					text: p.Handoff + ", " + L["branch"] + " `" + p.Branch + "`: " + strings.Join(fresh, "; ")}); e != nil {
					return "", nil, nil, e
				}
			}
			last := ""
			for _, d := range deltas {
				if d.Handoff != p.Handoff || !strings.HasPrefix(d.ID, "DELTA-") || deltaNumber(d.ID) <= markDelta {
					continue
				}
				last = d.ID
				ref := d.ID + ", " + p.Handoff
				body := sentence(d.Title) + " " + sentence(d.Detail)
				var r options
				switch {
				case !sourceRef.MatchString(d.Evidence):
					r = options{recKind: "question", text: ref + ": " + body, resolveBy: L["ask-evidence"]}
					review = append(review, map[string]string{"delta": d.ID, "reason": "no verifiable evidence"})
				case d.Type == "decision":
					r = options{recKind: "decision", text: body + " (" + d.Evidence + ")", by: L["development"] + ", " + ref}
				case d.Type == "question":
					r = options{recKind: "question", text: body, resolveBy: L["requester-or-cell"] + " (" + ref + ")"}
				default: // finding, verification, definition, deviation
					r = options{recKind: "evidence", level: "demonstrated", text: body, source: d.Evidence + " (" + ref + ")"}
					if d.Type == "definition" || d.Type == "deviation" {
						review = append(review, map[string]string{"delta": d.ID, "type": d.Type, "reason": "decide which requirement it supersedes (add --kind requirement --supersedes)"})
					}
				}
				if e := added(r); e != nil {
					return "", nil, nil, fmt.Errorf("%s: %v", d.ID, e)
				}
			}
			if len(fresh) == 0 && last == "" {
				continue
			}
			if last == "" {
				last = "—"
				if markDelta > 0 {
					last = fmt.Sprintf("DELTA-%03d", markDelta)
				}
			}
			mark := headSHA + " / " + last
			text = annotate(text, p.Handoff, L["reconciled"]+" "+mark)
			through[p.Handoff] = mark
			logParts = append(logParts, L["reconciliation"]+" "+p.Handoff+" → "+mark)
		}
		res := map[string]any{"records": records, "review": review, "through": through}
		if len(records) == 0 {
			res["next"] = "nothing new since the last reconciliation"
		} else if !o.apply {
			res["next"] = "review the records, then repeat with --apply"
		}
		if len(logParts) > 0 {
			logParts = []string{strings.Join(logParts, "; ") + " (" + strings.Join(records, ", ") + ")"}
		}
		return text, logParts, res, nil
	}, nil)
}
