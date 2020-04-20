package logger

type level int

func (l level) String() (str string) {
	switch l {
	case LevelInfo:
		str = "info"
	case LevelDebug:
		str = "debug"
	case LevelWarn:
		str = "warning"
	case LevelError:
		str = "error"
	case LevelFatal:
		str = "fatal"
	}
	return
}

// All known log levels
const (
	LevelDebug level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)
