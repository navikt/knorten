package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func main() {
	//data, err := os.ReadFile("noe.json")
	//if err != nil {
	//	panic("shiiit")
	//}

	// log := logrus.New()

	//repo, err := database.New("postgres://postgres:postgres@localhost:5432/knorten?sslmode=disable", log.WithField("subsystem", "repo"))
	//if err != nil {
	//	panic("setting up database")
	//}

	//ctx := context.Background()
	//values, err := repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, "nada")
	//if err != nil {
	//	panic("shiiit")
	//}

	//var d any
	//for _, v := range values {
	//	if strings.HasPrefix(v.Value, "[") || strings.HasPrefix(v.Value, "{") {
	//		if err := json.Unmarshal([]byte(v.Value), &d); err != nil {
	//			panic("shiiit")
	//		}
	//	}
	//}

	//users := []string{"erik.vattekar@nav.no", "other.person@nav.no"}
	//
	//quotified := make([]string, len(users))
	//for i, v := range users {
	//	quotified[i] = fmt.Sprintf("%q", v)
	//}
	//
	//fmt.Println(strings.Join(quotified, ", "))

	// value := "[{\"enabled\": {\"nesting\": false}, \"mountPath\":\"/etc/ssl/certs/ca-certificates.crt\",\"name\":\"ca-bundle-pem\",\"readOnly\":true,\"subPath\":\"ca-bundle.pem\"},{\"mountPath\":\"/etc/pki/tls/certs/ca-bundle.crt\",\"name\":\"ca-bundle-pem\",\"readOnly\":true,\"subPath\":\"ca-bundle.pem\"},{\"mountPath\":\"/etc/ssl/ca-bundle.pem,name:ca-bundle-pem\",\"readOnly\":true,\"subPath\":\"ca-bundle.pem\"},{\"mountPath\":\"/etc/pki/tls/cacert.pem\",\"name\":\"ca-bundle-pem\",\"readOnly\":true,\"subPath\":\"ca-bundle.pem\"},{\"mountPath\":\"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem\",\"name\":\"ca-bundle-pem\",\"readOnly\":true,\"subPath\":\"ca-bundle.pem\"}]"

	// value := "fase"

	//b, err := strconv.ParseBool(value)
	//if err != nil {
	//	panic(err)
	//}

	var d any
	err := json.Unmarshal([]byte("40fc85e1-6184-4134-9491-6c6b11bb5647"), &d)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(d)

	//_, _ = parseValue(value)

	//bytes, err := json.Marshal(res)
	//if err != nil {
	//	panic(err)
	//}
	//
	//err = os.WriteFile("out.json", bytes, fs.ModeAppend)
	//if err != nil {
	//	fmt.Println(err)
	//}
}

//func parseValue(value any) (any, error) {
//	var err error
//	switch v := value.(type) {
//	case string:
//		value, err = parseString(v)
//		if err != nil {
//			return nil, err
//		}
//	default:
//		fmt.Println(v)
//		value = v
//	}
//
//	return value, nil
//}
//
//func parseString(value any) (any, error) {
//	if strings.HasPrefix(value.(string), "[") || strings.HasPrefix(value.(string), "{") {
//		var d any
//		if err := json.Unmarshal([]byte(value.(string)), &d); err != nil {
//			return "", err
//		}
//		value = d
//	}
//	return value, nil
//}

func parseValue(value any) (any, error) {
	var err error
	switch v := value.(type) {
	case string:
		value, err = parseString(v)
		if err != nil {
			return nil, err
		}
	default:
		fmt.Println(v)
		value = v
	}

	return value, nil
}

func parseString(value any) (any, error) {
	var d any
	var err error
	if strings.HasPrefix(value.(string), "[") {
		if err := json.Unmarshal([]byte(value.(string)), &d); err != nil {
			return "", err
		}
		ret := make([]any, len(d.([]any)))
		for idx, elem := range d.([]any) {
			ret[idx], err = parseValue(elem)
			if err != nil {
				return nil, err
			}
		}
		value = ret
	} else if strings.HasPrefix(value.(string), "{") {
		if err := json.Unmarshal([]byte(value.(string)), &d); err != nil {
			return "", err
		}
		ret := map[string]any{}
		for key, val := range d.(map[string]any) {
			v, err := parseValue(val)
			if err != nil {
				return nil, err
			}
			ret[key] = v
		}
		value = ret
	}
	return value, nil
}
