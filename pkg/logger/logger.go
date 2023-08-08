package logger

type Logger interface {
	Errorf(string, ...any)
	Error(string ...any)
	Infof(string, ...any)
	Info(string ...any)

	WithError(error) Logger
	WithField(string, any) Logger
	WithTeamID(string) Logger
}
