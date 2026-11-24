// Package logger configures console and rotating file logs with request context.
package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level         string
	Outputs       []Output
	AddCaller     bool
	AddStacktrace string
}

type Output struct {
	Type       string
	Level      string
	Format     string
	Path       string
	Rotation   bool
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
}

type Logger struct {
	log         *zap.Logger
	atomicLevel *zap.AtomicLevel
	addCaller   bool
	files       []io.Closer
	borrowed    bool
	closeOnce   sync.Once
}

// ParseConfigFile reads an explicit TOML path.
func ParseConfigFile(path string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func NewLogger(config *Config) (*Logger, error) {
	if config == nil {
		return nil, errors.New("logger config is nil")
	}
	level := zapcore.InfoLevel
	if config.Level != "" {
		if err := level.UnmarshalText([]byte(config.Level)); err != nil {
			return nil, err
		}
	}
	atomicLevel := zap.NewAtomicLevelAt(level)
	l := &Logger{atomicLevel: &atomicLevel, addCaller: config.AddCaller}
	options := []zap.Option{zap.WithCaller(config.AddCaller), zap.AddCallerSkip(2)}
	if config.AddStacktrace != "" {
		var stacktraceLevel zapcore.Level
		if err := stacktraceLevel.UnmarshalText([]byte(config.AddStacktrace)); err != nil {
			return nil, err
		}
		options = append(options, zap.AddStacktrace(stacktraceLevel))
	}
	var cores []zapcore.Core
	for _, output := range config.Outputs {
		var enabled zapcore.LevelEnabler = atomicLevel
		if output.Level != "" {
			var outputLevel zapcore.Level
			if err := outputLevel.UnmarshalText([]byte(output.Level)); err != nil {
				l.Close()
				return nil, err
			}
			enabled = outputLevel
			if config.Level != "" {
				enabled = zap.LevelEnablerFunc(func(candidate zapcore.Level) bool {
					return outputLevel.Enabled(candidate) && atomicLevel.Enabled(candidate)
				})
			}
		}
		outputType := strings.ToLower(output.Type)
		format := strings.ToLower(output.Format)
		if format == "" {
			format = "console"
			if outputType == "file" {
				format = "json"
			}
		}
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		var encoder zapcore.Encoder
		switch format {
		case "json":
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		case "console":
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		default:
			l.Close()
			return nil, fmt.Errorf("unsupported log format: %s", output.Format)
		}
		var writer zapcore.WriteSyncer
		switch outputType {
		case "console", "stderr":
			writer = zapcore.Lock(zapcore.AddSync(os.Stderr))
		case "stdout":
			writer = zapcore.Lock(zapcore.AddSync(os.Stdout))
		case "file":
			if output.Path == "" {
				l.Close()
				return nil, errors.New("log file path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(output.Path), 0o755); err != nil {
				l.Close()
				return nil, err
			}
			if output.Rotation {
				file := &lumberjack.Logger{
					Filename:   output.Path,
					MaxSize:    output.MaxSize,
					MaxAge:     output.MaxAge,
					MaxBackups: output.MaxBackups,
					Compress:   output.Compress,
				}
				l.files = append(l.files, file)
				writer = zapcore.AddSync(file)
			} else {
				file, err := os.OpenFile(output.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					l.Close()
					return nil, err
				}
				l.files = append(l.files, file)
				writer = zapcore.Lock(zapcore.AddSync(file))
			}
		default:
			l.Close()
			return nil, fmt.Errorf("unsupported log output: %s", output.Type)
		}
		cores = append(cores, zapcore.NewCore(encoder, writer, enabled))
	}
	l.log = zap.New(zapcore.NewTee(cores...), options...)
	return l, nil
}

// SetLevel changes the base level shared by the logger and its children.
func (l *Logger) SetLevel(level string) error {
	if l == nil || l.atomicLevel == nil {
		return errors.New("logger has no configurable level")
	}
	var parsed zapcore.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		return err
	}
	l.atomicLevel.SetLevel(parsed)
	return nil
}

// Level returns the base level or the fixed level of an unconfigured logger.
func (l *Logger) Level() string {
	if l == nil || l.log == nil {
		return zapcore.InfoLevel.String()
	}
	if l.atomicLevel == nil {
		return zapcore.LevelOf(l.log.Core()).String()
	}
	return l.atomicLevel.Level().String()
}

// Close flushes logs and releases files. Call after all log writers have stopped.
func (l *Logger) Close() {
	if l == nil || l.borrowed {
		return
	}
	l.closeOnce.Do(func() {
		_ = l.Sync()
		for _, file := range l.files {
			_ = file.Close()
		}
	})
}

func (l *Logger) Sync() error {
	if l == nil || l.log == nil {
		return nil
	}
	return l.log.Sync()
}

func (l *Logger) write(ctx context.Context, level zapcore.Level, callerSkip int, format string, args ...any) {
	if l == nil || l.log == nil || !l.log.Core().Enabled(level) {
		return
	}
	log := l.log
	if callerSkip != 0 {
		log = log.WithOptions(zap.AddCallerSkip(callerSkip))
	}
	if entry := log.Check(level, fmt.Sprintf(format, args...)); entry != nil {
		entry.Write(contextFields(ctx)...)
	}
}

func contextFields(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}
	var fields []zap.Field
	if id, ok := ctx.Value("RequestID").(string); ok && id != "" {
		fields = append(fields, zap.String("requestId", id))
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		fields = append(
			fields,
			zap.String("traceId", span.TraceID().String()),
			zap.String("spanId", span.SpanID().String()),
		)
	}
	return fields
}

func (l *Logger) Debugf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.DebugLevel, 0, format, args...)
}

func (l *Logger) Infof(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.InfoLevel, 0, format, args...)
}

func (l *Logger) Warnf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.WarnLevel, 0, format, args...)
}

func (l *Logger) Errorf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.ErrorLevel, 0, format, args...)
}

func (l *Logger) Fatalf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.FatalLevel, 0, format, args...)
}

var (
	defaultMu     sync.RWMutex
	consoleLogger = &Logger{log: zap.NewExample()}
	defaultLogger = consoleLogger
)

func SetDefaultConfig(config *Config) error {
	next, err := NewLogger(config)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	previous := defaultLogger
	defaultLogger = next
	if previous != consoleLogger {
		previous.Close()
	}
	defaultMu.Unlock()
	return nil
}

func SetDefaultConfigWithFile(path string) error {
	config, err := ParseConfigFile(path)
	if err != nil {
		return err
	}
	return SetDefaultConfig(config)
}

func write(ctx context.Context, level zapcore.Level, format string, args ...any) {
	if l := contextualLogger(ctx); l != nil {
		l.write(ctx, level, 1, format, args...)
		return
	}
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	defaultLogger.write(ctx, level, 1, format, args...)
}

func Debugf(ctx context.Context, format string, args ...any) {
	write(ctx, zapcore.DebugLevel, format, args...)
}

func Infof(ctx context.Context, format string, args ...any) {
	write(ctx, zapcore.InfoLevel, format, args...)
}

func Warnf(ctx context.Context, format string, args ...any) {
	write(ctx, zapcore.WarnLevel, format, args...)
}

func Errorf(ctx context.Context, format string, args ...any) {
	write(ctx, zapcore.ErrorLevel, format, args...)
}

func Fatalf(ctx context.Context, format string, args ...any) {
	write(ctx, zapcore.FatalLevel, format, args...)
}

func Sync() error {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultLogger.Sync()
}
