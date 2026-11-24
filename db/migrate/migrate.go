// Package migrate runs versioned SQL and Go migrations using database/sql.
package migrate

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type Migration struct {
	Version int64
	Name    string
	UpSQL   string
	DownSQL string
	Up      func(context.Context, *sql.Tx) error
	Down    func(context.Context, *sql.Tx) error
}

// LoadFS loads paired SQL files from dir, ignoring entries with other suffixes.
func LoadFS(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	type files struct {
		migration Migration
		base      string
		up, down  bool
	}
	versions := make(map[int64]files)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		base, up := strings.CutSuffix(filename, ".up.sql")
		if !up {
			var down bool
			base, down = strings.CutSuffix(filename, ".down.sql")
			if !down {
				continue
			}
		}
		rawVersion, name, found := strings.Cut(base, "_")
		version, err := strconv.ParseInt(rawVersion, 10, 64)
		if !found || err != nil || version <= 0 || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid migration filename %q", filename)
		}
		pair, exists := versions[version]
		if exists && (pair.base != base || (up && pair.up) || (!up && pair.down)) {
			return nil, fmt.Errorf("duplicate migration version %d in %q", version, filename)
		}
		if !exists {
			pair.base = base
			pair.migration = Migration{Version: version, Name: name}
		}
		content, err := fs.ReadFile(fsys, path.Join(dir, filename))
		if err != nil {
			return nil, err
		}
		if up {
			pair.up, pair.migration.UpSQL = true, string(content)
		} else {
			pair.down, pair.migration.DownSQL = true, string(content)
		}
		versions[version] = pair
	}
	migrations := make([]Migration, 0, len(versions))
	for _, pair := range versions {
		if !pair.up {
			return nil, fmt.Errorf("migration %q is missing its .up.sql file", pair.base)
		}
		migrations = append(migrations, pair.migration)
	}
	sortMigrations(migrations)
	return migrations, nil
}

type StatusEntry struct {
	Version   int64
	Name      string
	Checksum  string
	Applied   bool
	AppliedAt time.Time
}

type Option func(*Runner)

// WithMigrations registers migrations. Multiple calls append to the same set.
func WithMigrations(migrations ...Migration) Option {
	return func(runner *Runner) {
		runner.migrations = append(runner.migrations, migrations...)
	}
}

// Runner borrows the supplied database pool. Coordinate migration runs across
// processes by assigning a single migration owner per database. Migration SQL
// must support the driver's transaction semantics. MySQL DDL can implicitly
// commit, so only transactional operations are suitable for atomic MySQL runs.
type Runner struct {
	db         *sql.DB
	migrations []Migration
	numbered   bool
	mutex      sync.Mutex
}

func New(sqlDB *sql.DB, dialect string, opts ...Option) (*Runner, error) {
	if sqlDB == nil {
		return nil, errors.New("migration database is nil")
	}
	runner := &Runner{db: sqlDB}
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql", "pgx", "gaussdb", "kingbase", "kingbasees":
		runner.numbered = true
	case "sqlite", "sqlite3", "mysql":
	default:
		return nil, fmt.Errorf("unsupported migration dialect %q", dialect)
	}
	for _, option := range opts {
		option(runner)
	}
	sortMigrations(runner.migrations)
	for index, migration := range runner.migrations {
		if migration.Version <= 0 || strings.TrimSpace(migration.Name) == "" {
			return nil, fmt.Errorf(
				"invalid migration version or name: %d %q",
				migration.Version,
				migration.Name,
			)
		}
		if index > 0 && runner.migrations[index-1].Version == migration.Version {
			return nil, fmt.Errorf("duplicate migration version %d", migration.Version)
		}
	}
	return runner, nil
}

func NewFromGORM(db *gorm.DB, opts ...Option) (*Runner, error) {
	if db == nil || db.Config == nil || db.Dialector == nil {
		return nil, errors.New("migration GORM database has no dialector")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	return New(sqlDB, db.Dialector.Name(), opts...)
}

func sortMigrations(migrations []Migration) {
	slices.SortFunc(migrations, func(left, right Migration) int {
		return cmp.Compare(left.Version, right.Version)
	})
}

// checksum hashes the exact up SQL. Assign Go callback changes a new version,
// since callback code does not have a portable content checksum.
func (migration Migration) checksum() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(migration.UpSQL)))
}

func (runner *Runner) ensureTable(ctx context.Context) error {
	_, err := runner.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TIMESTAMP NOT NULL
	)`)
	return err
}

type historyReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readHistory(ctx context.Context, reader historyReader) (map[int64]StatusEntry, error) {
	rows, err := reader.QueryContext(ctx, `SELECT version, name, checksum, applied_at FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := make(map[int64]StatusEntry)
	for rows.Next() {
		entry := StatusEntry{Applied: true}
		var appliedAt any
		if err := rows.Scan(&entry.Version, &entry.Name, &entry.Checksum, &appliedAt); err != nil {
			return nil, err
		}
		switch value := appliedAt.(type) {
		case time.Time:
			entry.AppliedAt = value
		case []byte:
			entry.AppliedAt, err = time.Parse(time.DateTime, string(value))
		case string:
			entry.AppliedAt, err = time.Parse(time.DateTime, value)
		default:
			err = fmt.Errorf("unsupported applied_at type %T", value)
		}
		if err != nil {
			return nil, fmt.Errorf("read migration %d timestamp: %w", entry.Version, err)
		}
		history[entry.Version] = entry
	}
	return history, rows.Err()
}

func (runner *Runner) verifyHistory(history map[int64]StatusEntry) error {
	known := 0
	for _, migration := range runner.migrations {
		if entry, exists := history[migration.Version]; exists {
			if entry.Checksum != migration.checksum() {
				return fmt.Errorf(
					"migration %d (%s) checksum mismatch",
					migration.Version,
					migration.Name,
				)
			}
			if entry.Name != migration.Name {
				return fmt.Errorf(
					"migration %d name mismatch: applied as %q",
					migration.Version,
					entry.Name,
				)
			}
			known++
		}
	}
	if len(history) != known {
		return errors.New("migration history contains versions absent from the registered migrations")
	}
	return nil
}

// Up checks the applied history before running each pending migration in its
// own transaction. If both forms are supplied, SQL runs before the callback.
func (runner *Runner) Up(ctx context.Context) error {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	if err := runner.ensureTable(ctx); err != nil {
		return err
	}
	history, err := readHistory(ctx, runner.db)
	if err != nil {
		return err
	}
	if err := runner.verifyHistory(history); err != nil {
		return err
	}
	insert := `INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)`
	if runner.numbered {
		insert = `INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES ($1, $2, $3, $4)`
	}
	for _, migration := range runner.migrations {
		if _, applied := history[migration.Version]; applied {
			continue
		}
		err := runner.transact(ctx, func(tx *sql.Tx) error {
			if err := runMigration(ctx, tx, migration.UpSQL, migration.Up); err != nil {
				return err
			}
			_, err := tx.ExecContext(
				ctx,
				insert,
				migration.Version,
				migration.Name,
				migration.checksum(),
				time.Now().UTC(),
			)
			return err
		})
		if err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}
	}
	return nil
}

// Down reverts up to steps applied migrations, newest first, in one transaction.
func (runner *Runner) Down(ctx context.Context, steps int) error {
	if steps < 0 {
		return errors.New("migration rollback steps must be non-negative")
	}
	if steps == 0 {
		return nil
	}
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	if err := runner.ensureTable(ctx); err != nil {
		return err
	}
	remove := `DELETE FROM schema_migrations WHERE version = ?`
	if runner.numbered {
		remove = `DELETE FROM schema_migrations WHERE version = $1`
	}
	return runner.transact(ctx, func(tx *sql.Tx) error {
		history, err := readHistory(ctx, tx)
		if err != nil {
			return err
		}
		if err := runner.verifyHistory(history); err != nil {
			return err
		}
		var rollback []Migration
		for index := len(runner.migrations) - 1; index >= 0 && len(rollback) < steps; index-- {
			migration := runner.migrations[index]
			if _, applied := history[migration.Version]; !applied {
				continue
			}
			if strings.TrimSpace(migration.DownSQL) == "" && migration.Down == nil {
				return fmt.Errorf(
					"migration %d (%s) has no down migration",
					migration.Version,
					migration.Name,
				)
			}
			rollback = append(rollback, migration)
		}
		for _, migration := range rollback {
			if err := runMigration(ctx, tx, migration.DownSQL, migration.Down); err != nil {
				return fmt.Errorf(
					"revert migration %d (%s): %w",
					migration.Version,
					migration.Name,
					err,
				)
			}
			if _, err := tx.ExecContext(ctx, remove, migration.Version); err != nil {
				return fmt.Errorf("remove migration %d from history: %w", migration.Version, err)
			}
		}
		return nil
	})
}

func (runner *Runner) Status(ctx context.Context) ([]StatusEntry, error) {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	if err := runner.ensureTable(ctx); err != nil {
		return nil, err
	}
	history, err := readHistory(ctx, runner.db)
	if err != nil {
		return nil, err
	}
	for _, migration := range runner.migrations {
		if _, applied := history[migration.Version]; !applied {
			history[migration.Version] = StatusEntry{
				Version: migration.Version, Name: migration.Name, Checksum: migration.checksum(),
			}
		}
	}
	status := make([]StatusEntry, 0, len(history))
	for _, entry := range history {
		status = append(status, entry)
	}
	slices.SortFunc(status, func(left, right StatusEntry) int { return cmp.Compare(left.Version, right.Version) })
	return status, nil
}

func (runner *Runner) transact(ctx context.Context, operation func(*sql.Tx) error) error {
	tx, err := runner.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := operation(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func runMigration(
	ctx context.Context,
	tx *sql.Tx,
	statement string,
	callback func(context.Context, *sql.Tx) error,
) error {
	if strings.TrimSpace(statement) != "" {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if callback != nil {
		return callback(ctx, tx)
	}
	return nil
}
