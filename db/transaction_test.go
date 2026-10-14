package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func transactionTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = connection.Close() })
	if err := database.Exec(
		"CREATE TABLE transaction_probe (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)",
	).Error; err != nil {
		t.Fatal(err)
	}
	return database
}

func TestTransactionLifecycle(t *testing.T) {
	database := transactionTestDB(t)
	businessErr := errors.New("business conflict")
	for _, tc := range []struct {
		name        string
		callbackErr error
		panicValue  string
		wantRows    int64
	}{
		{name: "commit", wantRows: 1}, {name: "callback rollback", callbackErr: businessErr}, {name: "panic rollback", panicValue: "panic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := database.Exec("DELETE FROM transaction_probe").Error; err != nil {
				t.Fatal(err)
			}
			var gotPanic any
			var err error
			func() {
				defer func() { gotPanic = recover() }()
				err = Transaction(
					context.Background(),
					database,
					TransactionOptions{},
					func(tx *DB) error {
						if err := tx.Exec(
							"INSERT INTO transaction_probe VALUES (1, 1)",
						).Error; err != nil {
							return err
						}
						if nestedErr := Transaction(
							context.Background(),
							tx,
							TransactionOptions{},
							func(*DB) error { return nil },
						); nestedErr == nil {
							t.Error("nested transaction accepted")
						}
						if tc.panicValue != "" {
							panic(tc.panicValue)
						}
						return tc.callbackErr
					},
				)
			}()
			if !errors.Is(err, tc.callbackErr) {
				t.Fatalf("error = %v", err)
			}
			if tc.panicValue != "" && gotPanic != tc.panicValue {
				t.Fatalf("panic = %v", gotPanic)
			}
			var rows int64
			if err := database.Table("transaction_probe").Count(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if rows != tc.wantRows {
				t.Fatalf("rows = %d", rows)
			}
		})
	}
}

type transactionTestDialect struct{ gorm.Dialector }

func (transactionTestDialect) Name() string { return "postgres" }

func TestTransactionRetryBounds(t *testing.T) {
	database := transactionTestDB(t)
	database.Dialector = transactionTestDialect{Dialector: database.Dialector}
	aborted := fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "40001"})
	t.Run("whole transaction", func(t *testing.T) {
		attempts := 0
		var previous *DB
		err := Transaction(
			context.Background(),
			database,
			TransactionOptions{Retry: &TransactionRetry{MaxAttempts: 3, Timeout: time.Second}},
			func(tx *DB) error {
				attempts++
				if previous == tx {
					t.Error("transaction handle reused")
				}
				previous = tx
				if err := tx.Exec("INSERT INTO transaction_probe VALUES (1, 1)").Error; err != nil {
					return err
				}
				if attempts < 3 {
					return aborted
				}
				return nil
			},
		)
		if err != nil || attempts != 3 {
			t.Fatalf("attempts=%d error=%v", attempts, err)
		}
	})
	for _, tc := range []struct {
		name           string
		attempts       int
		timeout, delay time.Duration
		cancel         bool
		want           error
	}{
		{name: "attempt budget", attempts: 2, timeout: time.Second, want: aborted},
		{name: "time budget", attempts: 10, timeout: 10 * time.Millisecond, delay: time.Hour, want: context.DeadlineExceeded},
		{name: "cancel wait", attempts: 10, timeout: time.Hour, delay: time.Hour, cancel: true, want: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			err := Transaction(
				ctx,
				database,
				TransactionOptions{
					Retry: &TransactionRetry{
						MaxAttempts: tc.attempts,
						Timeout:     tc.timeout,
						Delay:       tc.delay,
					},
				},
				func(*DB) error {
					calls++
					if tc.cancel {
						cancel()
					}
					return aborted
				},
			)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v", err)
			}
			if tc.want == aborted && calls != tc.attempts {
				t.Fatalf("attempts=%d", calls)
			}
			if tc.want != aborted && !errors.Is(err, aborted) {
				t.Fatalf("original error lost: %v", err)
			}
		})
	}
}

func TestTransactionCanceledCallback(t *testing.T) {
	database := transactionTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	err := Transaction(ctx, database, TransactionOptions{}, func(tx *DB) error {
		if err := tx.Exec("INSERT INTO transaction_probe VALUES (1, 1)").Error; err != nil {
			return err
		}
		cancel()
		return tx.Exec("INSERT INTO transaction_probe VALUES (2, 2)").Error
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestTransactionAborted(t *testing.T) {
	for _, tc := range []struct {
		dialect string
		err     error
		want    bool
	}{
		{dialect: "postgres", err: &pgconn.PgError{Code: "40001"}, want: true},
		{dialect: "postgres", err: &pgconn.PgError{Code: "40P01"}, want: true},
		{dialect: "postgres", err: &pgconn.PgError{Code: "23505"}, want: false},
		{dialect: "mysql", err: &mysql.MySQLError{Number: 1213}, want: true},
		{dialect: "mysql", err: &mysql.MySQLError{Number: 1205}, want: false},
		{dialect: "mysql", err: &mysql.MySQLError{Number: 1062}, want: false},
		{dialect: "sqlite", err: &pgconn.PgError{Code: "40001"}, want: false},
		{dialect: "postgres", err: errors.New("connection lost during commit"), want: false},
	} {
		if got := transactionAborted(tc.dialect, fmt.Errorf("wrapped: %w", tc.err)); got != tc.want {
			t.Errorf("%s %v: %v", tc.dialect, tc.err, got)
		}
	}
}

type failingCommitPool struct {
	gorm.ConnPool
	connection *sql.DB
	err        error
}

func (p failingCommitPool) BeginTx(ctx context.Context, options *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.connection.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &failingCommitTx{Tx: tx, err: p.err}, nil
}

type failingCommitTx struct {
	*sql.Tx
	err error
}

func (tx failingCommitTx) Commit() error { _ = tx.Tx.Rollback(); return tx.err }

func TestTransactionUnknownCommitResultDoesNotRetry(t *testing.T) {
	database := transactionTestDB(t)
	database.Dialector = transactionTestDialect{Dialector: database.Dialector}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	commitErr := errors.New("connection lost during commit")
	database.Statement.ConnPool = failingCommitPool{ConnPool: connection, connection: connection, err: commitErr}
	attempts := 0
	err = Transaction(
		context.Background(),
		database,
		TransactionOptions{Retry: &TransactionRetry{MaxAttempts: 3, Timeout: time.Second}},
		func(tx *DB) error {
			attempts++
			return tx.Exec("INSERT INTO transaction_probe VALUES (1, 1)").Error
		},
	)
	if !errors.Is(err, commitErr) || attempts != 1 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
}
