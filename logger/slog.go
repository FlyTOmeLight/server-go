package logger

import (
	"context"
	"log/slog"
	"runtime"
	"slices"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// With returns a logger with additional fields and borrows the parent's outputs.
// Arguments use slog's key-value and Attr conventions, and may include zap fields.
func (l *Logger) With(args ...any) *Logger {
	if l == nil || l.log == nil {
		return l
	}
	var fields []zap.Field
	for len(args) > 0 {
		head := args[0]
		args = args[1:]
		switch arg := head.(type) {
		case zap.Field:
			fields = append(fields, arg)
		case slog.Attr:
			fields = appendSlogAttr(fields, arg)
		case string:
			if len(args) == 0 {
				fields = append(fields, zap.String("!BADKEY", arg))
			} else {
				fields = appendSlogAttr(fields, slog.Any(arg, args[0]))
				args = args[1:]
			}
		default:
			fields = appendSlogAttr(fields, slog.Any("!BADKEY", arg))
		}
	}
	return &Logger{
		log:         l.log.With(fields...),
		atomicLevel: l.atomicLevel,
		addCaller:   l.addCaller,
		files:       l.files,
		borrowed:    true,
	}
}

// SlogHandler returns a handler backed by this logger and its configured outputs.
func (l *Logger) SlogHandler() slog.Handler {
	handler := slogHandler{group: &slogGroup{}}
	if l != nil && l.log != nil {
		handler.log = l.log.WithOptions(zap.WithCaller(false))
		handler.addCaller = l.addCaller
	}
	return handler
}

func (l *Logger) Slog() *slog.Logger {
	return slog.New(l.SlogHandler())
}

type slogHandler struct {
	log       *zap.Logger
	addCaller bool
	group     *slogGroup
}

type slogGroup struct {
	name   string
	fields []zap.Field
	parent *slogGroup
}

func (h slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.log != nil && h.log.Core().Enabled(zapLevel(level))
}

func (h slogHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.log == nil {
		return nil
	}
	entry := h.log.Check(zapLevel(record.Level), record.Message)
	if entry == nil {
		return nil
	}
	if !record.Time.IsZero() {
		entry.Time = record.Time
	}
	if h.addCaller && record.PC != 0 {
		frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
		entry.Caller = zapcore.NewEntryCaller(frame.PC, frame.File, frame.Line, frame.PC != 0)
	}
	var fields []zap.Field
	record.Attrs(func(attr slog.Attr) bool {
		fields = appendSlogAttr(fields, attr)
		return true
	})
	for group := h.group; group != nil; group = group.parent {
		fields = append(slices.Clone(group.fields), fields...)
		if group.name != "" && len(fields) > 0 {
			fields = []zap.Field{slogGroupField(group.name, fields)}
		}
	}
	entry.Write(append(fields, contextFields(ctx)...)...)
	return nil
}

func (h slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	group := *h.group
	group.fields = slices.Clone(group.fields)
	for _, attr := range attrs {
		group.fields = appendSlogAttr(group.fields, attr)
	}
	h.group = &group
	return h
}

func (h slogHandler) WithGroup(name string) slog.Handler {
	if name != "" {
		h.group = &slogGroup{name: name, parent: h.group}
	}
	return h
}

func appendSlogAttr(fields []zap.Field, attr slog.Attr) []zap.Field {
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		var members []zap.Field
		for _, member := range value.Group() {
			members = appendSlogAttr(members, member)
		}
		if attr.Key == "" {
			return append(fields, members...)
		}
		if len(members) > 0 {
			return append(fields, slogGroupField(attr.Key, members))
		}
		return fields
	}
	if attr.Equal(slog.Attr{}) {
		return fields
	}
	return append(fields, zap.Any(attr.Key, value.Any()))
}

func slogGroupField(name string, fields []zap.Field) zap.Field {
	return zap.Object(name, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
		for _, field := range fields {
			field.AddTo(encoder)
		}
		return nil
	}))
}

func zapLevel(level slog.Level) zapcore.Level {
	switch {
	case level >= slog.LevelError:
		return zapcore.ErrorLevel
	case level >= slog.LevelWarn:
		return zapcore.WarnLevel
	case level >= slog.LevelInfo:
		return zapcore.InfoLevel
	default:
		return zapcore.DebugLevel
	}
}
