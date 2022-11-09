package chart

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
)

func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func reflectValueToString(value reflect.Value) (string, error) {
	switch value.Type().Kind() {
	case reflect.String:
		return value.String(), nil
	case reflect.Slice:
		parts, ok := value.Interface().([]string)
		if !ok {
			return "", fmt.Errorf("unable to parse reflect slice: %v", value)
		}

		quotified := make([]string, len(parts))
		for i, v := range parts {
			quotified[i] = fmt.Sprintf("%q", v)
		}
		return fmt.Sprintf("[%v]", strings.Join(quotified, ", ")), nil
	default:
		return "", fmt.Errorf("helm value must be string or slice of strings, unable to parse helm value: %v", value)

	}
}

func createChartValues(values reflect.Value, fields []reflect.StructField) (map[string]string, error) {
	chartValues := map[string]string{}

	for _, field := range fields {
		tag := field.Tag.Get("helm")
		if tag == "" {
			continue
		}
		value := values.FieldByName(field.Name)
		valueAsString, err := reflectValueToString(value)
		if err != nil {
			return map[string]string{}, err
		}

		if valueAsString != "" {
			chartValues[tag] = valueAsString
		}
	}

	return chartValues, nil
}
