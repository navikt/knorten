package logger

import "github.com/sirupsen/logrus"

type Logger interface {
	Errorf(string, ...any)
	Error(string ...any)
	Infof(string, ...any)
	Info(string ...any)

	WithError(error) *logrus.Entry
}
