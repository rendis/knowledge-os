package investigation

import (
	"golang.org/x/text/unicode/norm"
	"regexp"
	"strings"
)

var titleTokens = regexp.MustCompile(`[a-z0-9]+`)

func normalizeTitle(s string) string {
	var ascii strings.Builder
	for _, r := range norm.NFKD.String(s) {
		if r < 128 {
			ascii.WriteRune(r)
		}
	}
	return strings.Join(titleTokens.FindAllString(strings.ToLower(ascii.String()), -1), "-")
}
