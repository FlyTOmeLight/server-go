package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// TransactionOptions selects SQL transaction semantics and optional whole-transaction retries.
// A nil Retry performs exactly one attempt.
type TransactionOptions struct {
	Isolation sql.IsolationLevel
	ReadOnly  bool
	Retry     *TransactionRetry
}

// TransactionRetry bounds all attempts, including callback execution and retry waits.
// MaxAttempts includes the first attempt; Timeout and MaxAttempts must be positive.
type TransactionRetry struct {
	MaxAttempts int
	Timeout     time.Duration
	Delay       time.Duration
}

// Transaction runs fn with a new GORM transaction bound to ctx. Callback errors and
// panics roll back; panics propagate. Nested calls are rejected, including when
// retries are disabled. Return database errors without converting their identity.
// A retry repeats the entire callback; keep external side effects outside it.
func Transaction(ctx context.Context, database *DB, options TransactionOptions, fn func(*DB) error) error {
	if database == nil || fn == nil {
		return errors.New("transaction requires a database and callback")
	}
	if _, nested := database.Statement.ConnPool.(gorm.TxCommitter); nested {
		return errors.New("nested Transaction calls are not supported; use the outer transaction handle")
	}
	attempts := 1
	delay := time.Duration(0)
	if retry := options.Retry; retry != nil {
		if retry.MaxAttempts < 1 || retry.Timeout <= 0 || retry.Delay < 0 {
			return errors.New(
				"transaction retry requires positive MaxAttempts and Timeout, and nonnegative Delay",
			)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, retry.Timeout)
		defer cancel()
		attempts, delay = retry.MaxAttempts, retry.Delay
	}
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := database.WithContext(ctx).
			Transaction(fn, &sql.TxOptions{Isolation: options.Isolation, ReadOnly: options.ReadOnly})
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return errors.Join(ctx.Err(), err)
		}
		if attempt == attempts || !transactionAborted(database.Dialector.Name(), err) {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
	}
}

// These server error codes guarantee the transaction was aborted. Lock timeouts,
// unique conflicts, and network/commit errors do not provide that guarantee.
func transactionAborted(dialect string, err error) bool {
	switch dialect {
	case "postgres":
		var serverErr *pgconn.PgError
		return errors.As(err, &serverErr) && (serverErr.Code == "40001" || serverErr.Code == "40P01")
	case "mysql":
		var serverErr *mysql.MySQLError
		return errors.As(err, &serverErr) && serverErr.Number == 1213
	default:
		return false
	}
}
