package db

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const maxTransactionRetryDelay = time.Second

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

	// Delay starts exponential backoff with jitter and a one-second cap. Zero retries immediately.
	Delay time.Duration
}

// Transaction runs fn with a new GORM transaction bound to ctx. Callback errors and
// panics roll back; panics propagate. Nested calls are rejected, including when
// retries are disabled. Return database errors without converting their identity.
// A retry repeats the entire callback; keep external side effects outside it.
func Transaction(ctx context.Context, database *DB, options TransactionOptions, fn func(*DB) error) error {
	if ctx == nil {
		return errors.New("transaction context is nil")
	}
	if !validTransactionDB(database) || fn == nil {
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
		attempts, delay = retry.MaxAttempts, min(retry.Delay, maxTransactionRetryDelay)
	}
	var lastErr error
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return errors.Join(err, lastErr)
			}
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
		if attempt == attempts || !transactionAborted(database, err) {
			return err
		}
		lastErr = err
		if delay == 0 {
			continue
		}
		// Spread concurrent retries across the last quarter of the backoff window.
		timer := time.NewTimer(delay - rand.N(delay/4+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
		delay = min(delay*2, maxTransactionRetryDelay)
	}
}

func validTransactionDB(database *DB) bool {
	return database != nil && database.Statement != nil && database.Config != nil && database.Dialector != nil
}

// These server error codes guarantee the transaction was aborted. Lock timeouts,
// unique conflicts, and network/commit errors do not provide that guarantee.
func transactionAborted(database *DB, err error) bool {
	if !validTransactionDB(database) {
		return false
	}
	switch database.Dialector.Name() {
	case "postgres", "gaussdb":
		var serverErr interface{ SQLState() string }
		if !errors.As(err, &serverErr) {
			return false
		}
		code := serverErr.SQLState()
		return code == "40001" || code == "40P01"
	case "mysql":
		var serverErr *mysql.MySQLError
		return errors.As(err, &serverErr) && serverErr.Number == 1213
	default:
		return false
	}
}
