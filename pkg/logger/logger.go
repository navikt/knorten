package logger

type Logger interface {
	Fatalf(string, ...any)
	Errorf(string, ...any)
	Infof(string, ...any)
	Debugf(string, ...any)
}
