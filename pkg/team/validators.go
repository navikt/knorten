package team

import (
	"net/mail"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var ValidateTeamName validator.Func = func(fl validator.FieldLevel) bool {
	teamSlug := fl.Field().Interface().(string)

	r, _ := regexp.Compile("^[a-z-]+$")
	return r.MatchString(teamSlug)
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
