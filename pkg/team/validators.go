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
	users, ok := fl.Field().Interface().([]string)
	if !ok {
		return false
	}

	for _, user := range users {
		if user == "" {
			continue
		}
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
