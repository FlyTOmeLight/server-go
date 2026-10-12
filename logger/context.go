package logger

import "context"

type contextKey struct{}

// WithContext attaches an explicitly owned logger to a request or worker context.
// The caller must close the logger after all users of the context have stopped.
func WithContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

func contextualLogger(ctx context.Context) *Logger {
	if ctx == nil {
		return nil
	}
	l, _ := ctx.Value(contextKey{}).(*Logger)
	return l
}

// FromContext returns the attached logger, or a fixed console logger when none
// was supplied. The console logger does not load configuration or create files.
func FromContext(ctx context.Context) *Logger {
	if l := contextualLogger(ctx); l != nil {
		return l
	}
	return consoleLogger
}
