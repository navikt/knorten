package user

import (
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

var ValidateDiskSize validator.Func = func(fl validator.FieldLevel) bool {
	diskSize := fl.Field().Interface().(string)

	diskSizeInt, err := strconv.Atoi(strings.TrimSpace(diskSize))
	if err != nil {
		return false
	}

	return diskSizeInt < 10 || diskSizeInt <= 200
}
