package logger

import "github.com/sirupsen/logrus"

type Logger interface {
	Errorf(string, ...any)
	Infof(string, ...any)
	Info(string ...any)

	WithError(error) *logrus.Entry
	WithField(string, any) *logrus.Entry
}
