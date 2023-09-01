package chart

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

var ValidateAirflowRepo validator.Func = func(fl validator.FieldLevel) bool {
	repoName := fl.Field().Interface().(string)

	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return false
	}

	return parts[0] == "navikt"
}

var ValidateRepoBranch validator.Func = func(fl validator.FieldLevel) bool {
	branch := fl.Field().Interface().(string)
	return !strings.Contains(branch, "/")
}

var ValidateAirflowImage validator.Func = func(fl validator.FieldLevel) bool {
	image := fl.Field().Interface().(string)

	imageParts := strings.Split(image, ":")
	if len(imageParts) != 2 {
		return false
	}

	if !strings.HasPrefix(imageParts[0], "ghcr.io/navikt/") && !strings.HasPrefix(imageParts[0], "europe-north1-docker.pkg.dev") {
		return false
	}

	return true
}
