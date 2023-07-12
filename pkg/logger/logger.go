package logger

type Logger interface {
	Errorf(string, ...any)
	Infof(string, ...any)
}
