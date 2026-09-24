package investigation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf16"
)

// pythonCanonical reproduces json.dumps formatting without replacing escape
// sequences inside strings. Literal backslash-u text must remain literal.
func pythonCanonical(v any, ascii, spaces bool) ([]byte, error) {
	encoded, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var decoded any
	d := json.NewDecoder(bytes.NewReader(encoded))
	d.UseNumber()
	if e = d.Decode(&decoded); e != nil {
		return nil, e
	}
	var b bytes.Buffer
	quote := func(s string) {
		b.WriteByte('"')
		for _, r := range s {
			switch r {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if r < 32 || (ascii && r > 126) {
					if r <= 0xffff {
						fmt.Fprintf(&b, "\\u%04x", r)
					} else {
						a, z := utf16.EncodeRune(r)
						fmt.Fprintf(&b, "\\u%04x\\u%04x", a, z)
					}
				} else {
					b.WriteRune(r)
				}
			}
		}
		b.WriteByte('"')
	}
	comma := func() {
		b.WriteByte(',')
		if spaces {
			b.WriteByte(' ')
		}
	}
	var emit func(any)
	emit = func(v any) {
		switch x := v.(type) {
		case nil:
			b.WriteString("null")
		case string:
			quote(x)
		case bool:
			fmt.Fprint(&b, x)
		case json.Number:
			b.WriteString(string(x))
		case []any:
			b.WriteByte('[')
			for i, y := range x {
				if i > 0 {
					comma()
				}
				emit(y)
			}
			b.WriteByte(']')
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			b.WriteByte('{')
			for i, k := range keys {
				if i > 0 {
					comma()
				}
				quote(k)
				b.WriteByte(':')
				if spaces {
					b.WriteByte(' ')
				}
				emit(x[k])
			}
			b.WriteByte('}')
		}
	}
	emit(decoded)
	return b.Bytes(), nil
}
