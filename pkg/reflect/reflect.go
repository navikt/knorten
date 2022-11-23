package reflect

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

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

func CreateChartValues(form any) (map[string]string, error) {
	values := reflect.ValueOf(form)
	fields := reflect.VisibleFields(reflect.TypeOf(form))

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

func InterfaceToStruct(obj any, values map[string]string) error {
	structValue := reflect.ValueOf(obj).Elem()
	fields := reflect.VisibleFields(structValue.Type())
	for _, field := range fields {
		fieldTag := field.Tag.Get("helm")
		value := values[fieldTag]

		structValue := reflect.ValueOf(obj).Elem()
		structFieldValue := structValue.FieldByName(field.Name)

		if !structFieldValue.IsValid() {
			return fmt.Errorf("no such field: %s in obj", field.Name)
		}

		if !structFieldValue.CanSet() {
			return fmt.Errorf("cannot set %s field value", field.Name)
		}

		kind := structFieldValue.Kind()
		switch kind {
		case reflect.String:
			structFieldValue.Set(reflect.ValueOf(value))
		case reflect.Slice:
			var slice []string
			err := json.Unmarshal([]byte(value), &slice)
			if err != nil {
				return err
			}
			structFieldValue.Set(reflect.ValueOf(slice))
		default:
			return fmt.Errorf("unknown kind('%v')", kind)
		}
	}

	return nil
}
