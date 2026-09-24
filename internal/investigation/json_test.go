package investigation

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestPythonJSONEscapingParity(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("reference Python unavailable")
	}
	for _, ascii := range []bool{false, true} {
		for _, spaces := range []bool{false, true} {
			value := map[string]any{"literal": `\u003c\u2028`, "unicode": "<>&\u2028\u2029ñ😀\x7f", "array": []string{"a", "b"}}
			actual, e := pythonCanonical(value, ascii, spaces)
			if e != nil {
				t.Fatal(e)
			}
			args := []string{"-c", `import json,sys;v=json.load(sys.stdin);sys.stdout.write(json.dumps(v,sort_keys=True,ensure_ascii=sys.argv[1]=='yes',separators=(', ', ': ') if sys.argv[2]=='yes' else (',',':')))`, "no", "no"}
			if ascii {
				args[2] = "yes"
			}
			if spaces {
				args[3] = "yes"
			}
			cmd := exec.Command(python, args...)
			cmd.Stdin = bytes.NewReader(actual)
			expected, e := cmd.Output()
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(expected, actual) {
				t.Fatalf("escape parity differs\n%q\n%q", actual, expected)
			}
		}
	}
}
