package migrate

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSQLite(t *testing.T) (*gorm.DB, *sql.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	return db, pool
}

func TestUpStatusAndDown(t *testing.T) {
	db, pool := openSQLite(t)
	first := Migration{
		Version: 1, Name: "items",
		UpSQL: "CREATE TABLE items (label TEXT)", DownSQL: "DROP TABLE items",
	}
	second := Migration{
		Version: 2, Name: "seed",
		Up: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "INSERT INTO items VALUES (?)", "seed")
			return err
		},
		Down: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM items WHERE label = ?", "seed")
			return err
		},
	}
	runner, err := NewFromGORM(db, WithMigrations(second, first))
	require.NoError(t, err)
	ctx := t.Context()
	status, err := runner.Status(ctx)
	require.NoError(t, err)
	require.Len(t, status, 2)
	require.EqualValues(t, 1, status[0].Version)
	require.False(t, status[0].Applied)
	require.True(t, status[0].AppliedAt.IsZero())
	require.NoError(t, runner.Up(ctx))
	require.NoError(t, runner.Up(ctx))

	var count int
	require.NoError(t, pool.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count))
	require.Equal(t, 1, count)
	status, err = runner.Status(ctx)
	require.NoError(t, err)
	require.True(t, status[0].Applied)
	require.False(t, status[0].AppliedAt.IsZero())
	require.True(t, status[1].Applied)

	require.NoError(t, runner.Down(ctx, 1))
	require.NoError(t, pool.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count))
	require.Zero(t, count)
	status, err = runner.Status(ctx)
	require.NoError(t, err)
	require.True(t, status[0].Applied)
	require.False(t, status[1].Applied)
	require.NoError(t, runner.Down(ctx, 1))
	require.Error(t, pool.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count))
}

func TestFailedUpRollsBackAndCanRetry(t *testing.T) {
	_, pool := openSQLite(t)
	failure := errors.New("callback failed")
	ctx := t.Context()
	broken := Migration{
		Version: 2, Name: "broken",
		UpSQL: "CREATE TABLE broken (id INT)",
		Up:    func(context.Context, *sql.Tx) error { return failure },
	}
	first := Migration{Version: 1, Name: "items", UpSQL: "CREATE TABLE items (id INT)"}
	runner, err := New(pool, "sqlite", WithMigrations(first, broken))
	require.NoError(t, err)
	require.ErrorIs(t, runner.Up(ctx), failure)
	status, err := runner.Status(ctx)
	require.NoError(t, err)
	require.True(t, status[0].Applied)
	require.False(t, status[1].Applied)

	broken.Up = nil
	retry, err := New(pool, "sqlite", WithMigrations(first, broken))
	require.NoError(t, err)
	require.NoError(t, retry.Up(ctx))

	first.UpSQL += " "
	drifted, err := New(pool, "sqlite", WithMigrations(first, broken, Migration{
		Version: 3, Name: "pending", UpSQL: "CREATE TABLE pending (id INT)",
	}))
	require.NoError(t, err)
	require.ErrorContains(t, drifted.Up(ctx), "checksum mismatch")
	status, err = drifted.Status(ctx)
	require.NoError(t, err)
	require.False(t, status[2].Applied)
}

func TestDownIsAtomic(t *testing.T) {
	_, pool := openSQLite(t)
	failure := errors.New("down callback failed")
	first := Migration{
		Version: 1, Name: "items", UpSQL: "CREATE TABLE items (id INT)", DownSQL: "DROP TABLE items",
		Down: func(context.Context, *sql.Tx) error { return failure },
	}
	second := Migration{
		Version: 2, Name: "seed", UpSQL: "INSERT INTO items VALUES (1)", DownSQL: "DELETE FROM items",
	}
	ctx := t.Context()
	runner, err := New(pool, "sqlite", WithMigrations(first, second))
	require.NoError(t, err)
	require.NoError(t, runner.Up(ctx))
	require.ErrorIs(t, runner.Down(ctx, 2), failure)

	first.DownSQL, first.Down = "", nil
	irreversible, err := New(pool, "sqlite", WithMigrations(first, second))
	require.NoError(t, err)
	require.ErrorContains(t, irreversible.Down(ctx, 2), "has no down migration")
	status, err := runner.Status(ctx)
	require.NoError(t, err)
	require.True(t, status[0].Applied)
	require.True(t, status[1].Applied)
	var count int
	require.NoError(t, pool.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count))
	require.Equal(t, 1, count)
}

func TestLoadFS(t *testing.T) {
	migrations, err := LoadFS(fstest.MapFS{
		"sql/10_seed.up.sql":     {Data: []byte("seed")},
		"sql/02_create.up.sql":   {Data: []byte("create")},
		"sql/02_create.down.sql": {Data: []byte("drop")},
		"sql/README.md":          {Data: []byte("ignored")},
	}, "sql")
	require.NoError(t, err)
	require.Equal(t, []Migration{
		{Version: 2, Name: "create", UpSQL: "create", DownSQL: "drop"},
		{Version: 10, Name: "seed", UpSQL: "seed"},
	}, migrations)

	for _, test := range []struct {
		name, want string
		files      []string
	}{
		{name: "zero", files: []string{"0_create.up.sql"}, want: "invalid"},
		{name: "negative", files: []string{"-1_create.up.sql"}, want: "invalid"},
		{name: "duplicate", files: []string{"01_create.up.sql", "1_replace.up.sql"}, want: "duplicate"},
		{name: "missing up", files: []string{"1_create.down.sql"}, want: "missing its .up.sql"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := fstest.MapFS{}
			for _, name := range test.files {
				fixture[name] = &fstest.MapFile{}
			}
			_, err := LoadFS(fixture, ".")
			require.ErrorContains(t, err, test.want)
		})
	}
}
