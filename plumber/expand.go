package main

import (
	"errors"
)

// Expand replaces ${var} and $var in s based on the vars map.
// Bytes can be escaped using backslash or a quote pair.
// Every escaping backslash or quotes are removed.
// An invalid syntax returns an empty string and an error.
func Expand(s []byte, vars *map[string]string) (string, error) {
	var	buf []byte
	var quoted bool
	var escaped bool

	i := 0
	for j := 0; j < len(s); j++ {
		if !escaped && s[j] == '\\' {
			escaped = true
			s = append(s[:j], s[j + 1:]...)
			j--
			continue
		}
		if (i == 0 || !escaped) && s[j] == '\'' {
			quoted = !quoted
			s = append(s[:j], s[j + 1:]...)
			j--
		}
		if !escaped && !quoted && s[j] == '$' {
			if buf == nil {
				buf = make([]byte, 0, 2*len(s))
			}
			buf = append(buf, s[i:j]...)
			varName, w := getVarName(s[j + 1:])
			if varName == "" {
				return "", errors.New("invalid syntax")
			} else {
				buf = append(buf, getVarValue(varName, vars)...)
			}
			j += w
			i = j + 1
		}
		escaped = false
	}
	if buf == nil {
		return string(s), nil
	}
	return string(buf) + string(s[i:]), nil
}

func getVarName(s []byte) (string, int) {
	if s[0] == '{' {
		for i := 1; i < len(s); i++ {
			if s[i] == '}' {
				if i == 1 {
					return "", 2
				}
				return string(s[1:i]), i + 1
			} else if !isAlphaNum(s[i]) {
				return "", 2
			}
		}
		return "", 2
	}
	var i int
	for i = 0; i < len(s) && isAlphaNum(s[i]); i++ {
	}
	return string(s[:i]), i
}

func getVarValue(s string, vars *map[string]string) []byte {
	value, exists := (*vars)[s]
	if !exists {
		value = ""
	}
	return []byte(value)
}

func isAlphaNum(c uint8) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}