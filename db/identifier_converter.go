package db

import (
	"context"
	"database/sql"
	"strings"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// ConnPool 包装器：在 SQL 发往数据库前自动转换反引号标识符
// ---------------------------------------------------------------------------
//
// 为什么用 ConnPool 包装而不是回调替换？
//
// GORM 的 Query / Create / Update / Delete 回调在内部同时完成 SQL 构建和执行，
// 直接调用 db.Statement.ConnPool.QueryContext / ExecContext，
// 不经过独立的 "gorm:raw" 回调。替换 "gorm:raw" 只对 db.Exec() 生效，
// 对模型查询（如 db.Find()）完全无效。
//
// ConnPool 包装器在所有 SQL 执行的最终出口拦截，无论 SQL 是如何构建的，
// 都能确保反引号被正确转换。
// ---------------------------------------------------------------------------

// identifierConverter 包装 gorm.ConnPool，在执行 SQL 前自动转换标识符引用符号。
type identifierConverter struct {
	inner     gorm.ConnPool // 原始连接池（通常是 *sql.DB）
	quoteChar byte          // 目标引用字符（'"' 或 '`'）
}

// newIdentifierConverter 创建标识符转换包装器。
func newIdentifierConverter(inner gorm.ConnPool, quoteChar byte) *identifierConverter {
	return &identifierConverter{inner: inner, quoteChar: quoteChar}
}

// convert 转换 SQL 中的反引号为目标引用字符。
func (w *identifierConverter) convert(sqlStr string) string {
	if w.quoteChar == '`' || sqlStr == "" || !strings.ContainsRune(sqlStr, '`') {
		return sqlStr
	}
	return convertSQLIdentifiersByQuote(sqlStr, w.quoteChar)
}

// ---- 实现 gorm.ConnPool 接口 ----

func (w *identifierConverter) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return w.inner.PrepareContext(ctx, w.convert(query))
}

func (w *identifierConverter) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return w.inner.ExecContext(ctx, w.convert(query), args...)
}

func (w *identifierConverter) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return w.inner.QueryContext(ctx, w.convert(query), args...)
}

func (w *identifierConverter) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return w.inner.QueryRowContext(ctx, w.convert(query), args...)
}

// ---- 实现 gorm.ConnPoolBeginner 接口 ----
// GORM 的 Begin() 方法通过此接口开启事务，返回的 ConnPool 会替换当前的 ConnPool，
// 使事务内的所有查询也经过标识符转换。

func (w *identifierConverter) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	// 从底层连接池获取 *sql.Tx
	beginner, ok := w.inner.(gorm.TxBeginner)
	if !ok {
		// 底层不支持事务，返回错误让 GORM 处理
		return nil, gorm.ErrInvalidTransaction
	}
	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	// 返回包装后的事务对象，确保事务内的 SQL 也被转换
	return newConverterTx(tx, w.quoteChar), nil
}

// ---- 实现 GetDBConnector 接口（PreparedStmt 兼容） ----

func (w *identifierConverter) GetDBConn() (*sql.DB, error) {
	if sqldb, ok := w.inner.(*sql.DB); ok {
		return sqldb, nil
	}
	if connector, ok := w.inner.(gorm.GetDBConnector); ok && connector != nil {
		return connector.GetDBConn()
	}
	return nil, gorm.ErrInvalidDB
}

// ---------------------------------------------------------------------------
// converterTx：事务版包装器
// ---------------------------------------------------------------------------

// converterTx 包装 *sql.Tx，在事务内自动转换标识符引用符号。
// 同时实现 gorm.ConnPool 和 gorm.TxCommitter，使 GORM 能正常提交/回滚事务。
type converterTx struct {
	inner     *sql.Tx
	quoteChar byte
}

func newConverterTx(tx *sql.Tx, quoteChar byte) *converterTx {
	return &converterTx{inner: tx, quoteChar: quoteChar}
}

func (t *converterTx) convert(sqlStr string) string {
	if t.quoteChar == '`' || sqlStr == "" || !strings.ContainsRune(sqlStr, '`') {
		return sqlStr
	}
	return convertSQLIdentifiersByQuote(sqlStr, t.quoteChar)
}

// ---- 实现 gorm.ConnPool 接口 ----

func (t *converterTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return t.inner.PrepareContext(ctx, t.convert(query))
}

func (t *converterTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.inner.ExecContext(ctx, t.convert(query), args...)
}

func (t *converterTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.inner.QueryContext(ctx, t.convert(query), args...)
}

func (t *converterTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.inner.QueryRowContext(ctx, t.convert(query), args...)
}

// ---- 实现 gorm.TxCommitter 接口（GORM Commit/Rollback 需要） ----

func (t *converterTx) Commit() error { return t.inner.Commit() }

func (t *converterTx) Rollback() error { return t.inner.Rollback() }

// ---------------------------------------------------------------------------
// setupIdentifierConverter：在 gorm.Open 后调用
// ---------------------------------------------------------------------------

// setupIdentifierConverter 包装 db.ConnPool，使所有 SQL 在执行前自动转换标识符。
//
// 原理：用 identifierConverter 包装原始 ConnPool，拦截 PrepareContext / ExecContext /
// QueryContext / QueryRowContext 四个方法，在转发前将反引号替换为目标引用符号。
//
// 事务支持：identifierConverter 实现 ConnPoolBeginner 接口，BeginTx 返回 converterTx
// （包装了 *sql.Tx），确保事务内的查询也被转换。
func setupIdentifierConverter(db *gorm.DB) {
	if db == nil || db.Dialector == nil {
		return
	}

	name := strings.ToLower(db.Dialector.Name())

	// 仅 PostgreSQL / 达梦 / 人大金仓 / GaussDB 需要转换，MySQL 等不处理
	if !needsIdentifierConvert(name) {
		return
	}

	qc := identifierQuoteChar(name)

	// 包装 db.ConnPool（Dialector 在 Initialize 时设置的，通常是 *sql.DB）
	if db.ConnPool == nil {
		return
	}
	wrapper := newIdentifierConverter(db.ConnPool, qc)
	db.ConnPool = wrapper

	// 同步包装 db.Statement.ConnPool（gorm.Open 在初始化末尾从 db.ConnPool 复制而来）
	if db.Statement != nil {
		db.Statement.ConnPool = wrapper
	}
}

// ---------------------------------------------------------------------------
// convertSQLIdentifiersByQuote：SQL 标识符转换核心逻辑
// ---------------------------------------------------------------------------

// convertSQLIdentifiersByQuote 将 SQL 中的反引号替换为目标引用字符，
// 同时跳过单引号/双引号包裹的字符串字面量。
func convertSQLIdentifiersByQuote(sqlStr string, quoteChar byte) string {
	if quoteChar == '`' {
		return sqlStr
	}

	var buf strings.Builder
	buf.Grow(len(sqlStr))

	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(sqlStr); i++ {
		ch := sqlStr[i]

		switch {
		case inSingleQuote:
			buf.WriteByte(ch)
			if ch == '\'' {
				inSingleQuote = false
			} else if ch == '\\' && i+1 < len(sqlStr) {
				buf.WriteByte(sqlStr[i+1])
				i++
			}

		case inDoubleQuote:
			buf.WriteByte(ch)
			if ch == '"' {
				inDoubleQuote = false
			} else if ch == '\\' && i+1 < len(sqlStr) {
				buf.WriteByte(sqlStr[i+1])
				i++
			}

		case ch == '\'':
			inSingleQuote = true
			buf.WriteByte(ch)

		case ch == '"':
			inDoubleQuote = true
			buf.WriteByte(ch)

		case ch == '`':
			buf.WriteByte(quoteChar)

		default:
			buf.WriteByte(ch)
		}
	}

	return buf.String()
}
