package config

import (
	"os"
	"regexp"
	"strings"
)

var (
	envRe          = regexp.MustCompile(`\$\{([^}]+)\}`)
	escapeDollarRe = regexp.MustCompile(`\\\$`)
)

func Expand(s string) string {
	const sentinel = "\x00ESCAPED_DOLLAR\x00"
	s = escapeDollarRe.ReplaceAllString(s, sentinel)

	out := envRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-1]
		key := inner
		def := ""
		hasDefault := false
		before, after, found := strings.Cut(inner, ":-")
		if found {
			key = before
			def = after
			hasDefault = true
		}
		if v, ok := os.LookupEnv(key); ok && v != "" {
			return v
		}
		if hasDefault {
			return def
		}
		return ""
	})

	return strings.ReplaceAll(out, sentinel, "$")
}

func ExpandMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = Expand(v)
	}
	return out
}
