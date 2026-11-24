package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/gaussdb"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/soft_delete"
)

type BaseDataWithTenant struct {
	Pk          int64   `gorm:"primaryKey;autoIncrement"`
	ID          string  `gorm:"column:id;uniqueIndex;type:varchar(32)"`
	Name        string  `gorm:"type:varchar(64);default:'';NOT NULL"`
	Description *string `gorm:"type:varchar(2049);default:'';NOT NULL"`

	TenantID string `gorm:"column:tenant_id;type:varchar(32);default:'';NOT NULL"`
	UserID   string `gorm:"column:user_id;type:varchar(32);default:'';NOT NULL"`

	CreatedAt UTCTime               `gorm:"NOT NULL"`
	UpdatedAt UTCTime               `gorm:"NOT NULL"`
	DeletedAt soft_delete.DeletedAt `gorm:"softDelete:milli;NOT NULL"`
}

type BaseData struct {
	Pk          int64   `gorm:"primaryKey;autoIncrement"`
	ID          string  `gorm:"column:id;uniqueIndex;type:varchar(32)"`
	Name        string  `gorm:"type:varchar(64);default:'';NOT NULL"`
	Description *string `gorm:"type:varchar(2049);default:'';NOT NULL"`

	ProjectID string `gorm:"column:project_id;type:varchar(32);index;default:'';NOT NULL"`
	UserID    string `gorm:"column:user_id;type:varchar(32);default:'';NOT NULL"`

	CreatedAt UTCTime               `gorm:"NOT NULL"`
	UpdatedAt UTCTime               `gorm:"NOT NULL"`
	DeletedAt soft_delete.DeletedAt `gorm:"softDelete:milli;NOT NULL"`
}

type BaseDataWithoutDeleted struct {
	Pk          int64   `gorm:"primaryKey;autoIncrement"`
	ID          string  `gorm:"column:id;uniqueIndex;type:varchar(32)"`
	Name        string  `gorm:"type:varchar(64);default:'';NOT NULL"`
	Description *string `gorm:"type:varchar(2049);default:'';NOT NULL"`

	ProjectID string `gorm:"column:project_id;type:varchar(32);index;default:'';NOT NULL"`
	UserID    string `gorm:"column:user_id;type:varchar(32);default:'';NOT NULL"`

	CreatedAt UTCTime `gorm:"NOT NULL"`
	UpdatedAt UTCTime `gorm:"NOT NULL"`
}

type DB = gorm.DB

func NewDB(ctx context.Context, config *Config) (*DB, error) {
	if config == nil {
		return nil, errors.New("database config is nil")
	}
	if err := config.check(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var db *DB
	var err error

	switch config.Driver {
	case DriverMySQL:
		db, err = newMySQL(config)
	case DriverSQLite:
		db, err = newSQLite(config)
	case DriverPostgreSQL:
		db, err = newPostgreSQL(config)
	case DriverClickHouse:
		db, err = newClickHouse(config)
	case DriverKingbase:
		db, err = newKingbase(config)
	case DriverGaussDB:
		db, err = newGaussDB(config)
	case "":
		db, err = newMySQL(config)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", config.Driver)
	}
	if err != nil {
		return nil, err
	}

	connection, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := connection.PingContext(ctx); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	if config.MaxIdleConns > 0 {
		connection.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.MaxOpenConns > 0 {
		connection.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.ConnMaxIdleTime > 0 {
		connection.SetConnMaxIdleTime(time.Duration(config.ConnMaxIdleTime) * time.Second)
	}
	if config.ConnMaxLifeTime > 0 {
		connection.SetConnMaxLifetime(time.Duration(config.ConnMaxLifeTime) * time.Second)
	}

	return db, err
}

func mysqlDSN(config *Config) string {
	dsn := mysqldriver.NewConfig()
	dsn.User = config.User
	dsn.Passwd = config.Password
	dsn.Net = "tcp"
	dsn.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	dsn.DBName = config.DB
	dsn.ParseTime = true
	dsn.Loc = time.Local
	dsn.InterpolateParams = true
	if config.DSNParams == "" {
		dsn.Params = map[string]string{"charset": "utf8mb4"}
		return dsn.FormatDSN()
	}

	// FormatDSN already writes a query string because ParseTime is set.
	return dsn.FormatDSN() + "&" + config.DSNParams
}

func newMySQL(config *Config) (*DB, error) {
	db, err := gorm.Open(NewAIBaseDialector("mysql", mysql.Open(mysqlDSN(config))), &gorm.Config{
		Logger:                                   gormLogger{level: gormlogger.Info},
		DisableAutomaticPing:                     true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	return db, err
}

func newSQLite(config *Config) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(config.DB), &gorm.Config{
		Logger:                                   gormLogger{level: gormlogger.Info},
		DisableAutomaticPing:                     true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	return db, err
}

func quoteLibpqValue(value string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, "'", `\'`).Replace(value) + "'"
}

func libpqDSN(config *Config) string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		quoteLibpqValue(config.Host), quoteLibpqValue(config.User),
		quoteLibpqValue(config.Password), quoteLibpqValue(config.DB), config.Port)
}

func postgresDSN(config *Config) string {
	if config.DSN != "" {
		return config.DSN
	}
	dsn := libpqDSN(config) + " TimeZone=Asia/Shanghai"
	if config.DSNParams != "" {
		dsn += " " + config.DSNParams
	}
	return dsn
}

func newPostgreSQL(config *Config) (*DB, error) {
	db, err := gorm.Open(NewAIBaseDialector("postgres", postgres.New(postgres.Config{
		DSN:                  postgresDSN(config),
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	})), &gorm.Config{DisableAutomaticPing: true, Logger: gormLogger{level: gormlogger.Info}})
	return db, err
}

func clickhouseDSN(config *Config) string {
	query := url.Values{
		"database": {config.DB},
		"username": {config.User},
		"password": {config.Password},
	}
	dsn := url.URL{
		Scheme:   "tcp",
		Host:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		RawQuery: query.Encode(),
	}
	return dsn.String()
}

func newClickHouse(config *Config) (*DB, error) {
	db, err := gorm.Open(
		clickhouse.Open(clickhouseDSN(config)),
		&gorm.Config{DisableAutomaticPing: true, Logger: gormLogger{level: gormlogger.Info}},
	)
	return db, err
}

func gaussdbDSN(config *Config) string {
	dsn := libpqDSN(config) + " TimeZone=Asia/Shanghai"
	if config.DSNParams != "" {
		return dsn + " " + config.DSNParams
	}
	return dsn + " application_name=server-go client_encoding=utf8"
}

func newGaussDB(config *Config) (*DB, error) {
	// GaussDB 为 PostgreSQL 兼容协议，直接使用 gaussdb 方言
	db, err := gorm.Open(NewAIBaseDialectorWithIntegerBoolean("gaussdb", gaussdb.New(gaussdb.Config{
		DSN:                  gaussdbDSN(config),
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	})), &gorm.Config{
		Logger:                                   gormLogger{level: gormlogger.Info},
		DisableAutomaticPing:                     true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	return db, err
}

func kingbaseDSN(config *Config) string {
	dsn := libpqDSN(config)
	if config.DSNParams != "" {
		return dsn + " " + config.DSNParams
	}
	return dsn + " TimeZone=Asia/Shanghai application_name=server-go client_encoding=utf8"
}

func newKingbase(config *Config) (*gorm.DB, error) {
	pgDialector := postgres.New(postgres.Config{
		DSN:                  kingbaseDSN(config),
		PreferSimpleProtocol: true,
	})

	// Kingbase 使用 PostgreSQL 兼容模式，并统一按整数绑定布尔参数。
	baseDialector := NewAIBaseDialectorWithIntegerBoolean("postgres", pgDialector)

	// 使用 PostgreSQL 方言连接 KingbaseES（人大金仓 PG 兼容模式）
	db, err := gorm.Open(baseDialector, &gorm.Config{
		Logger:                                   gormLogger{level: gormlogger.Info},
		DisableAutomaticPing:                     true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}
