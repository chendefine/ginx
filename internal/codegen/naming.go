package codegen

import (
	"strings"
	"unicode"
)

var commonInitialisms = map[string]bool{
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"OS":    true,
	"QPS":   true,
	"RAM":   true,
	"RPC":   true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"UUID":  true,
	"VM":    true,
	"XML":   true,
	"YAML":  true,
}

func ToCamelCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		upper := strings.ToUpper(w)
		if commonInitialisms[upper] {
			words[i] = upper
		} else {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, "")
}

// ToIdentifier 将字符串转为合法的 Go 导出标识符. 与 ToCamelCase 相同, 但当结果以数字开头时加 X 前缀.
func ToIdentifier(s string) string {
	result := ToCamelCase(s)
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "X" + result
	}
	return result
}

func splitWords(s string) []string {
	var words []string
	var current []rune

	flush := func() {
		if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '_' || r == '-' || r == '.' || r == '/' || r == '{' || r == '}' || r == ' ':
			flush()
		case unicode.IsUpper(r):
			if len(current) > 0 && !unicode.IsUpper(current[len(current)-1]) {
				flush()
			} else if len(current) > 1 && unicode.IsUpper(current[len(current)-1]) {
				if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					flush()
				}
			}
			current = append(current, r)
		default:
			current = append(current, r)
		}
	}
	flush()
	return words
}

func OperationName(method, path, operationID string) string {
	if operationID != "" {
		return ToIdentifier(operationID)
	}
	name := ToCamelCase(strings.ToLower(method))
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, seg := range segments {
		seg = strings.Trim(seg, "{}")
		if seg != "" {
			name += ToCamelCase(seg)
		}
	}
	return name
}
