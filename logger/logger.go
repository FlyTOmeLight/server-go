// Package logger configures console and rotating file logs with request context.
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level   string
	Outputs []Output
}

type Output struct {
	Type       string
	Level      string
	Path       string
	Rotation   bool
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
}

type Logger struct {
	log   *zap.Logger
	files []io.Closer
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
		return nil, fmt.Errorf("logger config is nil")
	}
	level := zapcore.InfoLevel
	if config.Level != "" {
		if err := level.UnmarshalText([]byte(config.Level)); err != nil {
			return nil, err
		}
	}
	l := &Logger{}
	var cores []zapcore.Core
	for _, output := range config.Outputs {
		outputLevel := level
		if output.Level != "" {
			if err := outputLevel.UnmarshalText([]byte(output.Level)); err != nil {
				l.Close()
				return nil, err
			}
		}
		if outputLevel < level && config.Level != "" {
			outputLevel = level
		}
		var writer zapcore.WriteSyncer
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder := zapcore.NewJSONEncoder(encoderConfig)
		switch strings.ToLower(output.Type) {
		case "console":
			writer = zapcore.Lock(zapcore.AddSync(os.Stderr))
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		case "file":
			if output.Path == "" {
				l.Close()
				return nil, fmt.Errorf("log file path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(output.Path), 0755); err != nil {
				l.Close()
				return nil, err
			}
			if output.Rotation {
				file := &lumberjack.Logger{Filename: output.Path, MaxSize: output.MaxSize, MaxAge: output.MaxAge, MaxBackups: output.MaxBackups, Compress: output.Compress}
				l.files = append(l.files, file)
				writer = zapcore.AddSync(file)
			} else {
				file, err := os.OpenFile(output.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
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
		cores = append(cores, zapcore.NewCore(encoder, writer, outputLevel))
	}
	l.log = zap.New(zapcore.NewTee(cores...))
	return l, nil
}

// Close flushes logs and releases files. Call after all log writers have stopped.
func (l *Logger) Close() {
	if l.log != nil {
		_ = l.log.Sync()
	}
	for _, file := range l.files {
		_ = file.Close()
	}
}

func (l *Logger) Sync() error { return l.log.Sync() }

func (l *Logger) write(ctx context.Context, level zapcore.Level, format string, args ...any) {
	if entry := l.log.Check(level, fmt.Sprintf(format, args...)); entry != nil {
		var fields []zap.Field
		if ctx != nil {
			if id, ok := ctx.Value("RequestID").(string); ok && id != "" {
				fields = append(fields, zap.String("requestId", id))
			}
		}
		entry.Write(fields...)
	}
}

func (l *Logger) Debugf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.DebugLevel, format, args...)
}
func (l *Logger) Infof(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.InfoLevel, format, args...)
}
func (l *Logger) Warnf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.WarnLevel, format, args...)
}
func (l *Logger) Errorf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.ErrorLevel, format, args...)
}
func (l *Logger) Fatalf(ctx context.Context, format string, args ...any) {
	l.write(ctx, zapcore.FatalLevel, format, args...)
}

var (
	defaultMu     sync.RWMutex
	defaultLogger = &Logger{log: zap.NewExample()}
)

func SetDefaultConfig(config *Config) error {
	next, err := NewLogger(config)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	previous := defaultLogger
	defaultLogger = next
	previous.Close()
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
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	defaultLogger.write(ctx, level, format, args...)
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
