package admin

import (
	"html/template"
	"strings"
)

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"dict": func(values ...any) (map[string]any, error) {
			m := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errDictKey
				}
				m[key] = values[i+1]
			}
			return m, nil
		},
		"matchOns": func() []string {
			return matchOns
		},
		"matchTypes": func() []string {
			return matchTypes
		},
		"headerName": func(v *string) string {
			if v == nil {
				return ""
			}
			return *v
		},
		"flag": countryFlag,
	}
}

func countryFlag(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return ""
	}
	return string(rune(0x1F1E6)+rune(code[0]-'A')) + string(rune(0x1F1E6)+rune(code[1]-'A'))
}

type dictKeyError struct{}

func (dictKeyError) Error() string { return "dict keys must be strings" }

var errDictKey = dictKeyError{}
