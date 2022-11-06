package chart

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
)

func reflectValueToString(value reflect.Value) (string, error) {
	valueString := ""
	switch value.Type().Kind() {
	case reflect.String:
		valueString = value.String()
	case reflect.Slice:
		ok := false
		parts, ok := value.Interface().([]string)
		if !ok {
			return "", fmt.Errorf("unable to parse reflect slice: %v", value)
		}

		quotified := make([]string, len(parts))
		for i, v := range parts {
			quotified[i] = fmt.Sprintf("%q", v)
		}
		valueString = fmt.Sprintf("[%v]", strings.Join(quotified, ", "))
	default:
		return "", fmt.Errorf("helm value must be string or slice of strings, unable to parse helm value: %v", value)
	}

	return valueString, nil
}

func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
