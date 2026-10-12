package db

import "strings"

// needsIdentifierConvert 判断指定数据库类型是否需要做标识符引用符号转换。
//
// 仅以下数据库需要转换（反引号 → 双引号）：
//   - PostgreSQL（postgres / postgresql）
//   - 达梦（dameng / dm）
//   - 人大金仓（kingbase）
//   - GaussDB（gaussdb）
//
// MySQL、SQLite 等使用反引号的数据库不转换。
func needsIdentifierConvert(dbType string) bool {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql", "dameng", "dm", "kingbase", "gaussdb":
		return true
	default:
		return false
	}
}

// identifierQuoteChar 返回指定数据库类型的标识符引用字符。
//
//   - postgres/dameng/kingbase/gaussdb → " (双引号)
//   - mysql/sqlite/其他         → ` (反引号)
func identifierQuoteChar(dbType string) byte {
	if needsIdentifierConvert(dbType) {
		return '"'
	}
	return '`'
}

// ConvertSQLIdentifiers 将 SQL 中反引号(`)引用的标识符（表名、列名等）
// 转换为目标数据库的引用符号。
//
// 转换规则：
//   - MySQL：保持反引号 ` 不变
//   - PostgreSQL / 达梦 / 人大金仓 / GaussDB：反引号 → 双引号 "
//
// 函数通过状态机遍历，能正确跳过单引号包裹的字符串字面量，
// 不会误改字符串内的反引号字符。
//
// 示例：
//
//	ConvertSQLIdentifiers("SELECT `name` FROM `user` WHERE `id` = 'hello`world'", "postgres")
//	→ `SELECT "name" FROM "user" WHERE "id" = 'hello`world'`
func ConvertSQLIdentifiers(sql string, dbType string) string {
	quoteChar := identifierQuoteChar(dbType)

	// 目标就是反引号 → 无需转换
	if quoteChar == '`' {
		return sql
	}

	var buf strings.Builder
	buf.Grow(len(sql))

	inSingleQuote := false // 是否在单引号字符串内
	inDoubleQuote := false // 是否在双引号标识符内

	for i := 0; i < len(sql); i++ {
		ch := sql[i]

		switch {
		case inSingleQuote:
			buf.WriteByte(ch)
			// 处理转义的单引号 '' 和 \'
			if ch == '\'' {
				inSingleQuote = false
			} else if ch == '\\' && i+1 < len(sql) {
				buf.WriteByte(sql[i+1])
				i++
			}

		case inDoubleQuote:
			buf.WriteByte(ch)
			if ch == '"' {
				inDoubleQuote = false
			} else if ch == '\\' && i+1 < len(sql) {
				buf.WriteByte(sql[i+1])
				i++
			}

		case ch == '\'':
			inSingleQuote = true
			buf.WriteByte(ch)

		case ch == '"':
			inDoubleQuote = true
			buf.WriteByte(ch)

		case ch == '`':
			// 反引号 → 目标引用字符
			buf.WriteByte(quoteChar)

		default:
			buf.WriteByte(ch)
		}
	}

	return buf.String()
}

// ConvertSQLIdentifiers 将当前 SQL 中的标识符引用符号转换为本 Dialector
// 对应数据库的格式。是包级函数 ConvertSQLIdentifiers 的便捷方法。
func (d *AIBaseDialector) ConvertSQLIdentifiers(sql string) string {
	return ConvertSQLIdentifiers(sql, d.dbType)
}
