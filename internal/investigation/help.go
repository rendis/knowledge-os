package investigation

import (
	"fmt"
	"io"
	"strings"
)

var commandOptions = map[string]string{
	"open":           "id title objective dedupe-key source-ref source-type requester-role export-intent purpose vault-outcome learning-outcome request-summary timestamp visibility",
	"load":           "id",
	"snapshot":       "case-dir",
	"list":           "",
	"validate":       "",
	"retire":         "id expected-public-sha256 reason source dependency-review absorption-review summary snapshot-commit destination authorized timestamp",
	"publish":        "id expected-public-sha256 expected-tree-sha256 retain-local source timestamp",
	"consolidate":    "canonical retire expected-canonical-sha256 expected-retire-sha256 timestamp",
	"transition":     "id to reason source blocked-on expected-public-sha256 timestamp",
	"close":          "id decision reason limitations source evidence expected-public-sha256 timestamp",
	"bind":           "id observation expected-public-sha256 expected-story-sha256",
	"save":           "id public-candidate expected-public-sha256 private-root private-candidate delete-private expected-private-sha256 source target private-target timestamp",
	"save-resources": "id candidate-dir expected-tree-sha256 expected-candidate-tree-sha256 target source timestamp",
}

func help(out io.Writer, verb string) error {
	if options, ok := commandOptions[verb]; ok {
		_, e := fmt.Fprintf(out, "vaultctl investigation --root <vault>/investigations %s [options]\nOptions: --root", verb)
		if e != nil {
			return e
		}
		for _, k := range strings.Fields(options) {
			if _, e = fmt.Fprintf(out, " --%s", k); e != nil {
				return e
			}
		}
		_, e = fmt.Fprintln(out, "\nRun uses reviewed SHA-256 values for compare-and-swap; writes require effective Git identity.\nRepeat --source-ref, --target, --private-target, --evidence, --destination or --retain-local to pass multiple values where supported.")
		return e
	}
	_, e := fmt.Fprintln(out, "vaultctl investigation --root <vault>/investigations <command> [options]\nCommands: open load list snapshot validate save save-resources publish transition close bind retire consolidate\nUse <command> --help for its options. Markdown remains the case authority; SQLite is not used for lifecycle storage.")
	return e
}
