package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTransactionDatabases(t *testing.T) {
	for _, backend := range []struct {
		name, env string
		open      func(string) gorm.Dialector
	}{
		{name: "postgres", env: "SERVER_GO_TEST_POSTGRES_DSN", open: postgres.Open},
		{name: "mysql", env: "SERVER_GO_TEST_MYSQL_DSN", open: mysql.Open},
	} {
		t.Run(backend.name, func(t *testing.T) {
			dsn := os.Getenv(backend.env)
			if dsn == "" {
				t.Skip(backend.env + " not set; make check requires both databases")
			}
			database, err := gorm.Open(
				backend.open(dsn),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
			)
			if err != nil {
				t.Fatal("database connection failed")
			}
			connection, err := database.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = connection.Close() })
			if err := database.Exec(
				"CREATE TABLE transaction_probe (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)",
			).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = database.Exec("DROP TABLE transaction_probe").Error })
			t.Run("commit rollback panic", func(t *testing.T) {
				for _, mode := range []string{"commit", "error", "panic"} {
					t.Run(mode, func(t *testing.T) {
						if err := database.Exec(
							"DELETE FROM transaction_probe",
						).Error; err != nil {
							t.Fatal(err)
						}
						sentinel := errors.New("business conflict")
						var gotPanic any
						var err error
						func() {
							defer func() { gotPanic = recover() }()
							err = Transaction(
								context.Background(),
								database,
								TransactionOptions{Isolation: sql.LevelReadCommitted},
								func(tx *DB) error {
									if err := tx.Exec(
										"INSERT INTO transaction_probe VALUES (1, 1)",
									).Error; err != nil {
										return err
									}
									if mode == "error" {
										return sentinel
									}
									if mode == "panic" {
										panic(sentinel)
									}
									return nil
								},
							)
						}()
						if mode == "error" && !errors.Is(err, sentinel) {
							t.Fatalf("error=%v", err)
						}
						if mode == "commit" && err != nil {
							t.Fatal(err)
						}
						if mode == "panic" && gotPanic != sentinel {
							t.Fatalf("panic=%v", gotPanic)
						}
						var rows int64
						if err := database.Table("transaction_probe").
							Count(&rows).
							Error; err != nil {
							t.Fatal(err)
						}
						want := int64(0)
						if mode == "commit" {
							want = 1
						}
						if rows != want {
							t.Fatalf("rows=%d", rows)
						}
					})
				}
			})

			t.Run("read committed sees later commit", func(t *testing.T) {
				if err := database.Exec("DELETE FROM transaction_probe").Error; err != nil {
					t.Fatal(err)
				}
				if err := database.Exec(
					"INSERT INTO transaction_probe VALUES (1, 1)",
				).Error; err != nil {
					t.Fatal(err)
				}
				err := Transaction(
					context.Background(),
					database,
					TransactionOptions{Isolation: sql.LevelReadCommitted},
					func(tx *DB) error {
						var before, after int
						if err := tx.Raw("SELECT value FROM transaction_probe WHERE id = 1").
							Scan(&before).
							Error; err != nil {
							return err
						}
						// This independent connection acts as another committed writer.
						if err := database.Exec(
							"UPDATE transaction_probe SET value = 2 WHERE id = 1",
						).Error; err != nil {
							return err
						}
						if err := tx.Raw("SELECT value FROM transaction_probe WHERE id = 1").
							Scan(&after).
							Error; err != nil {
							return err
						}
						if before != 1 || after != 2 {
							t.Errorf("values before=%d after=%d", before, after)
						}
						return nil
					},
				)
				if err != nil {
					t.Fatal(err)
				}
			})
			t.Run("read only", func(t *testing.T) {
				attempts := 0
				err := Transaction(
					context.Background(),
					database,
					TransactionOptions{
						Isolation: sql.LevelReadCommitted,
						ReadOnly:  true,
						Retry:     &TransactionRetry{MaxAttempts: 3, Timeout: time.Second},
					},
					func(tx *DB) error {
						attempts++
						return tx.Exec("INSERT INTO transaction_probe VALUES (3, 3)").Error
					},
				)
				if err == nil || attempts != 1 {
					t.Fatalf("attempts=%d error=%v", attempts, err)
				}
			})
			t.Run("unique conflict does not retry", func(t *testing.T) {
				if err := database.Exec("DELETE FROM transaction_probe").Error; err != nil {
					t.Fatal(err)
				}
				if err := database.Exec(
					"INSERT INTO transaction_probe VALUES (1, 1)",
				).Error; err != nil {
					t.Fatal(err)
				}
				attempts := 0
				err := Transaction(
					context.Background(),
					database,
					TransactionOptions{
						Retry: &TransactionRetry{MaxAttempts: 3, Timeout: time.Second},
					},
					func(tx *DB) error {
						attempts++
						return tx.Exec("INSERT INTO transaction_probe VALUES (1, 1)").Error
					},
				)
				if err == nil || attempts != 1 {
					t.Fatalf("attempts=%d error=%v", attempts, err)
				}
			})
			t.Run("cancel active query", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				query := "SELECT pg_sleep(10)"
				if backend.name == "mysql" {
					query = "SELECT SLEEP(10)"
				}
				err := Transaction(
					ctx,
					database,
					TransactionOptions{},
					func(tx *DB) error { return tx.Exec(query).Error },
				)
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("error=%v", err)
				}
			})
			t.Run("deadlock retries whole transactions", func(t *testing.T) {
				if err := database.Exec("DELETE FROM transaction_probe").Error; err != nil {
					t.Fatal(err)
				}
				if err := database.Exec(
					"INSERT INTO transaction_probe VALUES (1, 0), (2, 0)",
				).Error; err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				var ready sync.WaitGroup
				ready.Add(2)
				barrier := make(chan struct{})
				go func() { ready.Wait(); close(barrier) }()
				type result struct {
					attempts int
					err      error
				}
				results := make(chan result, 2)
				for first := 1; first <= 2; first++ {
					go func(first int) {
						attempts := 0
						err := Transaction(
							ctx,
							database,
							TransactionOptions{
								Isolation: sql.LevelReadCommitted,
								Retry: &TransactionRetry{
									MaxAttempts: 3,
									Timeout:     10 * time.Second,
									Delay:       time.Millisecond,
								},
							},
							func(tx *DB) error {
								attempts++
								err := tx.Exec(
									"UPDATE transaction_probe SET value = value + 1 WHERE id = ?",
									first,
								).Error
								if attempts == 1 {
									ready.Done()
									if err != nil {
										return err
									}
									select {
									case <-barrier:
									case <-ctx.Done():
										return ctx.Err()
									}
								}
								if err != nil {
									return err
								}
								return tx.Exec(
									"UPDATE transaction_probe SET value = value + 1 WHERE id = ?",
									3-first,
								).Error
							},
						)
						results <- result{attempts: attempts, err: err}
					}(first)
				}
				total := 0
				for range 2 {
					select {
					case got := <-results:
						if got.err != nil {
							t.Fatal(got.err)
						}
						total += got.attempts
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
				}
				if total != 3 {
					t.Fatalf("attempts=%d; expected one aborted attempt", total)
				}
				var values []int
				if err := database.Table("transaction_probe").
					Order("id").
					Pluck("value", &values).
					Error; err != nil {
					t.Fatal(err)
				}
				if len(values) != 2 || values[0] != 2 || values[1] != 2 {
					t.Fatalf("values=%v", values)
				}
			})
		})
	}
}
