package db

import (
	"context"
	"time"

	gormlogger "gorm.io/gorm/logger"

	"github.com/FlyTOmeLight/server-go/logger"
)

// gormLogger uses the logger carried by each operation's context, rather than
// storing a request or application logger on a shared database connection.
type gormLogger struct {
	level gormlogger.LogLevel
}

var _ gormlogger.Interface = gormLogger{}

func (l gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.level = level
	return l
}

func (l gormLogger) Info(ctx context.Context, message string, args ...any) {
	if l.level >= gormlogger.Info {
		logger.FromContext(ctx).Infof(ctx, message, args...)
	}
}

func (l gormLogger) Warn(ctx context.Context, message string, args ...any) {
	if l.level >= gormlogger.Warn {
		logger.FromContext(ctx).Warnf(ctx, message, args...)
	}
}

func (l gormLogger) Error(ctx context.Context, message string, args ...any) {
	if l.level >= gormlogger.Error {
		logger.FromContext(ctx).Errorf(ctx, message, args...)
	}
}

func (l gormLogger) Trace(ctx context.Context, begin time.Time, query func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	log := logger.FromContext(ctx)
	switch {
	case err != nil && l.level >= gormlogger.Error:
		sql, rows := query()
		log.Errorf(ctx, "sql=%s rows=%d duration=%s error=%v", sql, rows, elapsed, err)
	case elapsed > 200*time.Millisecond && l.level >= gormlogger.Warn:
		sql, rows := query()
		log.Warnf(ctx, "slow sql=%s rows=%d duration=%s", sql, rows, elapsed)
	case l.level >= gormlogger.Info:
		sql, rows := query()
		log.Infof(ctx, "sql=%s rows=%d duration=%s", sql, rows, elapsed)
	}
}
