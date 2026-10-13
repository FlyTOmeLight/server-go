package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/clickhouse"
	"gorm.io/driver/gaussdb"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/soft_delete"
)

type BaseDataWithOrg struct {
	Pk          int64   `gorm:"primaryKey;autoIncrement"`
	ID          string  `gorm:"column:id;uniqueIndex;type:varchar(32)"`
	Name        string  `gorm:"type:varchar(64);default:'';NOT NULL"`
	Description *string `gorm:"type:varchar(2049);default:'';NOT NULL"`

	OrgID  string `gorm:"column:org_id;type:varchar(32);default:'';NOT NULL"`
	UserID string `gorm:"column:user_id;type:varchar(32);default:'';NOT NULL"`

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
	connection.SetMaxIdleConns(config.MaxIdleConns)
	connection.SetMaxOpenConns(config.MaxOpenConns)
	connection.SetConnMaxIdleTime(time.Duration(config.ConnMaxIdleTime) * time.Second)
	connection.SetConnMaxLifetime(time.Duration(config.ConnMaxLifeTime) * time.Second)

	return db, err
}

func newMySQL(config *Config) (*DB, error) {
	formatter := "%s:%s@(%s:%d)/%s?"
	if config.DSNParams != "" {
		formatter += config.DSNParams
	} else {
		formatter += "charset=utf8&parseTime=True&loc=Local&interpolateParams=true"
	}

	args := fmt.Sprintf(formatter, config.User, config.Password, config.Host, config.Port, config.DB)

	db, err := gorm.Open(NewAIBaseDialector("mysql", mysql.Open(args)), &gorm.Config{
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

func newPostgreSQL(config *Config) (*DB, error) {
	formatter := "host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai"
	args := config.DSN
	if args == "" {
		args = fmt.Sprintf(formatter, config.Host, config.User, config.Password, config.DB, config.Port)
	}

	db, err := gorm.Open(NewAIBaseDialector("postgres", postgres.New(postgres.Config{
		DSN:                  args,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	})), &gorm.Config{DisableAutomaticPing: true, Logger: gormLogger{level: gormlogger.Info}})
	return db, err
}

func newClickHouse(config *Config) (*DB, error) {
	formatter := "tcp://%s:%d?database=%s&username=%s&password=%s"
	args := fmt.Sprintf(formatter, config.Host, config.Port, config.DB, config.User, config.Password)

	db, err := gorm.Open(
		clickhouse.Open(args),
		&gorm.Config{DisableAutomaticPing: true, Logger: gormLogger{level: gormlogger.Info}},
	)
	return db, err
}

func newGaussDB(config *Config) (*DB, error) {
	formatter := "host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai"
	if config.DSNParams != "" {
		formatter += " " + config.DSNParams
	} else {
		formatter += " TimeZone=Asia/Shanghai application_name=server-go client_encoding=utf8"
	}
	dsn := fmt.Sprintf(formatter, config.Host, config.User, config.Password, config.DB, config.Port)

	// GaussDB 为 PostgreSQL 兼容协议，直接使用 gaussdb 方言
	db, err := gorm.Open(NewAIBaseDialectorWithIntegerBoolean("gaussdb", gaussdb.New(gaussdb.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	})), &gorm.Config{
		Logger:                                   gormLogger{level: gormlogger.Info},
		DisableAutomaticPing:                     true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	return db, err
}

func newKingbase(config *Config) (*gorm.DB, error) {
	formatter := "host=%s user=%s password=%s dbname=%s port=%d sslmode=disable"
	if config.DSNParams != "" {
		formatter += " " + config.DSNParams
	} else {
		formatter += " TimeZone=Asia/Shanghai application_name=server-go client_encoding=utf8"
	}
	// 构建金仓连接串（PG 兼容模式）
	dsn := fmt.Sprintf(formatter, config.Host, config.User, config.Password, config.DB, config.Port)

	pgDialector := postgres.New(postgres.Config{
		DSN:                  dsn,
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
