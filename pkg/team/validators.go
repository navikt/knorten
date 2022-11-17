package team

import (
	"github.com/go-playground/validator/v10"
	"net/mail"
	"strings"
)

var ValidateTeamName validator.Func = func(fl validator.FieldLevel) bool {
	name := fl.Field().String()

	if len(name) < 6 {
		return false
	}

	return true
}

var ValidateTeamUsers validator.Func = func(fl validator.FieldLevel) bool {
	users := fl.Field().Interface().([]string)
	for _, user := range users {
		_, err := mail.ParseAddress(user)
		if err != nil {
			return false
		}
		if !strings.HasSuffix(strings.ToLower(user), "nav.no") {
			return false
		}
	}

	return true
}
