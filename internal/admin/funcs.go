package admin

import "html/template"

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
	}
}

type dictKeyError struct{}

func (dictKeyError) Error() string { return "dict keys must be strings" }

var errDictKey = dictKeyError{}
