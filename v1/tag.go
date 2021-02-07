package validate

import (
	"reflect"
	"strconv"
	"strings"
)

func getTag(tag reflect.StructTag, key string) string {
	v, _ := findTag(tag, key)
	return v
}

func findTag(tag reflect.StructTag, key string) (string, bool) {
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}

		qvalue := string(tag[:i+1])
		tag = tag[i+1:]
		for _, e := range strings.Split(name, ",") {
			if key == strings.TrimSpace(e) {
				value, err := strconv.Unquote(qvalue)
				if err != nil {
					break
				}
				return value, true
			}
		}
	}
	return "", false

}
