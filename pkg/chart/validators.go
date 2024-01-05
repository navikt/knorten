package chart

import (
	"regexp"
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

	if image == "" {
		return true
	}

	imageParts := strings.Split(image, ":")
	if len(imageParts) != 2 {
		return false
	}

	return strings.HasPrefix(imageParts[0], "ghcr.io/navikt/") || strings.HasPrefix(imageParts[0], "europe-north1-docker.pkg.dev")
}

var ValidateCPUSpec validator.Func = func(fl validator.FieldLevel) bool {
	CPUSpec := fl.Field().Interface().(string)
	// 1 || 1m || 1.0
	r, _ := regexp.Compile(`^(([0-9]+m?)|([0-9]+\.[0-9]+))$`)
	return r.MatchString(CPUSpec)
}

var ValidateMemorySpec validator.Func = func(fl validator.FieldLevel) bool {
	memorySpec := fl.Field().Interface().(string)
	// The memory resource is measured in bytes. You can express memory as a plain integer
	// or a fixed-point integer with one of these suffixes: E, P, T, G, M, K, Ei, Pi, Ti, Gi, Mi, Ki.
	r, _ := regexp.Compile(`^[0-9]+(E|P|T|G|M|K|Ei|Pi|Ti|Gi|Mi|Ki)?$`)
	return r.MatchString(memorySpec)
}
