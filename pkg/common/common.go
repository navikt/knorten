package common

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func GenerateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

type ErrRetry struct {
	msg string
	err error
}

func (e *ErrRetry) Error() string {
	return fmt.Sprintf("%s: %v", e.msg, e.err)
}

func (e *ErrRetry) Unwrap() error {
	return e.err
}

func NewErrRetry(err error) *ErrRetry {
	return &ErrRetry{
		msg: "retry due to error",
		err: err,
	}
}
