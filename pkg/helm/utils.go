package helm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func KeySplitHandleEscape(key string) []string {
	escape := false
	keys := strings.FieldsFunc(key, func(r rune) bool {
		if r == '\\' {
			escape = true
		} else if escape {
			escape = false
			return false
		}
		return r == '.'
	})

	var keysWithoutEscape []string
	for _, k := range keys {
		keysWithoutEscape = append(keysWithoutEscape, strings.ReplaceAll(k, "\\", ""))
	}

	return keysWithoutEscape
}

func SetChartValue(keys []string, value any, chart map[string]any) {
	key := keys[0]
	if len(keys) > 1 {
		if _, ok := chart[key].(map[string]any); !ok {
			chart[key] = map[string]any{}
		}
		SetChartValue(keys[1:], value, chart[key].(map[string]any))
		return
	}

	chart[key] = value
}

func ParseValue(value any) (any, error) {
	var err error

	switch v := value.(type) {
	case string:
		value, err = ParseString(v)
		if err != nil {
			fmt.Println("parsing value", v)
			return nil, err
		}
	default:
		value = v
	}

	return value, nil
}

func ParseString(value any) (any, error) {
	valueString := value.(string)

	if d, err := strconv.ParseBool(valueString); err == nil {
		return d, nil
	} else if d, err := strconv.ParseInt(valueString, 10, 64); err == nil {
		return d, nil
	} else if d, err := strconv.ParseFloat(valueString, 64); err == nil {
		return d, nil
	} else if strings.HasPrefix(value.(string), "[") || strings.HasPrefix(value.(string), "{") {
		var d any
		if err := json.Unmarshal([]byte(valueString), &d); err != nil {
			return nil, err
		}
		return d, nil
	}

	return removeQuotations(valueString), nil
}

func removeQuotations(s string) string {
	s = strings.TrimPrefix(s, "\"")
	return strings.TrimSuffix(s, "\"")
}
