package less

// logger is lib/less/logger.js: messages go to the listeners lessc registers.
type logger struct {
	onWarn  func(string)
	onInfo  func(string)
	onError func(string)
}

var lessLogger = &logger{}

func (l *logger) warn(msg string) {
	if l.onWarn != nil {
		l.onWarn(msg)
	}
}

func (l *logger) info(msg string) {
	if l.onInfo != nil {
		l.onInfo(msg)
	}
}

func (l *logger) error(msg string) {
	if l.onError != nil {
		l.onError(msg)
	}
}
