package db

import (
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// AIBaseDialector 包装任意 gorm.Dialector，根据 gorm 标签中的 type 做跨数据库类型转换
type AIBaseDialector struct {
	gorm.Dialector // ← 内嵌接口
	dbType         string
	integerBoolean bool
}

// NewAIBaseDialector 创建包装 Dialector。
// dbType 建议传: "mysql" / "postgres" / "gaussdb" / "dm"（大小写不敏感）。
func NewAIBaseDialector(dbType string, dialector gorm.Dialector) *AIBaseDialector {
	return &AIBaseDialector{Dialector: dialector, dbType: dbType}
}

// NewAIBaseDialectorWithIntegerBoolean 创建使用整数存储布尔值的包装 Dialector。
// 该模式用于 Kingbase 的 PostgreSQL 兼容连接，覆盖匿名结构体、JOIN 和原生 SQL 条件。
func NewAIBaseDialectorWithIntegerBoolean(dbType string, dialector gorm.Dialector) *AIBaseDialector {
	return &AIBaseDialector{Dialector: dialector, dbType: dbType, integerBoolean: true}
}

// GetDialector 返回内嵌的原始方言。
// 供需要拿到具体方言类型的驱动使用（如达梦驱动的 Create 回调需要读取 GormMode，
// 会通过该方法逐层剥离包装器）。
func (d *AIBaseDialector) GetDialector() gorm.Dialector {
	return d.Dialector
}

// GetDBType 返回包装时指定的数据库类型标识
func (d *AIBaseDialector) GetDBType() string {
	return d.dbType
}

// SavePoint 将 GORM 的保存点操作透传给底层方言。
// AIBaseDialector 通过嵌入 gorm.Dialector 包装底层方言，但嵌入接口不会提升
// SavePointerDialectorInterface 的额外方法，必须显式转发，否则嵌套事务会被
// GORM 判定为不支持的驱动。
func (d *AIBaseDialector) SavePoint(tx *gorm.DB, name string) error {
	savePointer, ok := d.Dialector.(gorm.SavePointerDialectorInterface)
	if !ok {
		return gorm.ErrUnsupportedDriver
	}
	return savePointer.SavePoint(tx, name)
}

// RollbackTo 将 GORM 的保存点回滚操作透传给底层方言。
func (d *AIBaseDialector) RollbackTo(tx *gorm.DB, name string) error {
	savePointer, ok := d.Dialector.(gorm.SavePointerDialectorInterface)
	if !ok {
		return gorm.ErrUnsupportedDriver
	}
	return savePointer.RollbackTo(tx, name)
}

// DataTypeOf 是唯一需要实现的方法
func (d *AIBaseDialector) DataTypeOf(field *schema.Field) string {
	// 1. 获取 gorm 标签中的 type
	gormType := getGormTypeFromTag(field)
	if gormType == "" {
		// 没有指定 type，使用原始 Dialector 的映射
		return d.Dialector.DataTypeOf(field)
	}

	// 2. 根据数据库类型映射（dbType 统一转小写比较，避免 "PostgreSQL" 这类常量不匹配）
	switch strings.ToLower(d.dbType) {
	case "mysql":
		return mapToMysql(gormType, field.AutoIncrement)
	case "postgres", "postgresql", "gaussdb":
		return mapToPostgres(gormType, field.AutoIncrement)
	case "dameng", "dm":
		return mapToDameng(gormType)
	default:
		return gormType
	}
}

// BindVarTo 将整数布尔模式下的 Go bool 参数绑定为 0/1。
// PostgreSQL/GaussDB 驱动默认会把 bool 编码为 t/f，而 Kingbase 兼容字段使用 int 存储。
func (d *AIBaseDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, value interface{}) {
	if strings.EqualFold(d.dbType, "postgres") || strings.EqualFold(d.dbType, "postgresql") || strings.EqualFold(d.dbType, "gaussdb") {
		if boolean, ok := value.(bool); ok && (d.integerBoolean || indexBooleanField(stmt, len(stmt.Vars)-1) != nil) {
			if boolean {
				stmt.Vars[len(stmt.Vars)-1] = 1
			} else {
				stmt.Vars[len(stmt.Vars)-1] = 0
			}
		}
	}
	d.Dialector.BindVarTo(writer, stmt, value)
}

func indexBooleanField(stmt *gorm.Statement, variableIndex int) *schema.Field {
	if stmt == nil || stmt.Schema == nil || variableIndex < 0 {
		return nil
	}

	sql := stmt.SQL.String()
	if columns := insertColumns(sql); len(columns) > 0 {
		return integerBooleanField(stmt.Schema, columns[variableIndex%len(columns)])
	}
	if columns := updateColumns(sql); variableIndex < len(columns) {
		return integerBooleanField(stmt.Schema, columns[variableIndex])
	}
	return integerBooleanField(stmt.Schema, currentConditionColumn(sql))
}

func integerBooleanField(statementSchema *schema.Schema, column string) *schema.Field {
	column = normalizeSQLIdentifier(column)
	if column == "" {
		return nil
	}
	field := statementSchema.LookUpField(column)
	if field == nil || !isBooleanField(field) {
		return nil
	}
	typeName := strings.ToLower(strings.TrimSpace(field.TagSettings["TYPE"]))
	for _, integerType := range []string{"int", "integer", "smallint", "bigint"} {
		if typeName == integerType || strings.HasPrefix(typeName, integerType+"(") {
			return field
		}
	}
	return nil
}

func isBooleanField(field *schema.Field) bool {
	fieldType := field.FieldType
	for fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	return fieldType.Kind() == reflect.Bool
}

func insertColumns(sql string) []string {
	valuesIndex := strings.Index(strings.ToUpper(sql), " VALUES ")
	if valuesIndex < 0 {
		return nil
	}
	columnsStart := strings.Index(sql, "(")
	columnsEnd := strings.LastIndex(sql[:valuesIndex], ")")
	if columnsStart < 0 || columnsEnd <= columnsStart {
		return nil
	}
	return strings.Split(sql[columnsStart+1:columnsEnd], ",")
}

func currentConditionColumn(sql string) string {
	trimmed := strings.TrimSpace(sql)
	if equalIndex := strings.LastIndex(trimmed, "="); equalIndex >= 0 {
		return lastSQLIdentifier(strings.TrimSpace(trimmed[:equalIndex]))
	}
	upper := strings.ToUpper(trimmed)
	if inIndex := strings.LastIndex(upper, " IN "); inIndex >= 0 {
		return lastSQLIdentifier(strings.TrimSpace(trimmed[:inIndex]))
	}
	return ""
}

func updateColumns(sql string) []string {
	upperSQL := strings.ToUpper(sql)
	setIndex := strings.Index(upperSQL, " SET ")
	if setIndex < 0 {
		return nil
	}
	assignments := sql[setIndex+len(" SET "):]
	if whereIndex := strings.Index(strings.ToUpper(assignments), " WHERE "); whereIndex >= 0 {
		assignments = assignments[:whereIndex]
	}

	parts := strings.Split(assignments, ",")
	columns := make([]string, 0, len(parts))
	for _, assignment := range parts {
		if equalIndex := strings.Index(assignment, "="); equalIndex >= 0 {
			columns = append(columns, strings.TrimSpace(assignment[:equalIndex]))
		}
	}
	return columns
}

func lastSQLIdentifier(value string) string {
	for index := len(value) - 1; index >= 0; index-- {
		if value[index] == ' ' || value[index] == '(' || value[index] == ')' {
			return value[index+1:]
		}
	}
	return value
}

func normalizeSQLIdentifier(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"`")
	if dot := strings.LastIndex(value, "."); dot >= 0 {
		value = value[dot+1:]
	}
	return strings.Trim(value, "\"`")
}

// Migrator 必须重写，否则类型转换在建表/迁移时不会生效！
//
// 原因：postgres/mysql/gaussdb/达梦驱动的 Migrator(db) 会把驱动"自己"塞进 Migrator 内部的
// Config.Dialector，而 GORM 建表、加列、改列时生成列类型走的是这个"内部 Dialector"
// （migrator.Migrator.DataTypeOf → m.Dialector.DataTypeOf），会完全绕过外层包装。
// 所以这里用反射把 Migrator 内嵌的 gorm.Dialector 字段替换为 AIBaseDialector 本身，
// 让所有迁移路径都经过上面的类型转换逻辑。
func (d *AIBaseDialector) Migrator(db *gorm.DB) gorm.Migrator {
	m := d.Dialector.Migrator(db)

	v := reflect.ValueOf(m)
	switch v.Kind() {
	case reflect.Ptr:
		if !v.IsNil() {
			swapMigratorDialector(v.Elem(), d)
		}
		return m
	case reflect.Struct:
		// 驱动返回的是结构体值（不可寻址），复制到可寻址变量里修改后再返回
		nv := reflect.New(v.Type()).Elem()
		nv.Set(v)
		swapMigratorDialector(nv, d)
		return nv.Interface().(gorm.Migrator)
	default:
		return m
	}
}

// swapMigratorDialector 递归查找 Migrator 结构里内嵌的 gorm.Dialector 接口字段
// （即 migrator.Config 中的匿名字段），找到后替换为包装器。
func swapMigratorDialector(v reflect.Value, d *AIBaseDialector) bool {
	if v.Kind() != reflect.Struct {
		return false
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if !t.Field(i).Anonymous {
			continue
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Interface:
			if inner, ok := fv.Interface().(gorm.Dialector); ok && inner != nil {
				if _, isWrapper := inner.(*AIBaseDialector); !isWrapper {
					fv.Set(reflect.ValueOf(d))
					return true
				}
			}
		case reflect.Struct:
			if swapMigratorDialector(fv, d) {
				return true
			}
		}
	}
	return false
}

// 其他方法自动继承自内嵌的 gorm.Dialector，不需要写！

// ========== 工具函数 ==========

func getGormTypeFromTag(field *schema.Field) string {
	gormTag := field.Tag.Get("gorm")
	if gormTag == "" {
		return ""
	}

	parts := strings.Split(gormTag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "type:") {
			return strings.TrimPrefix(part, "type:")
		}
	}
	return ""
}

// ========== 类型映射函数（简化版） ==========

func mapToPostgres(gormType string, autoIncrement bool) string {
	switch strings.ToLower(strings.TrimSpace(gormType)) {
	case "smallint":
		if autoIncrement {
			return "smallserial"
		}
		return gormType
	case "int", "integer":
		if autoIncrement {
			return "serial"
		}
		return gormType
	case "bigint":
		if autoIncrement {
			return "bigserial"
		}
		return gormType
	case "tinyint", "tinyint unsigned":
		return "BOOLEAN"
	case "datetime":
		return "TIMESTAMPTZ"
	case "json":
		return "JSONB"
	case "longtext", "mediumtext":
		return "TEXT"
	default:
		return gormType
	}
}

func mapToMysql(gormType string, autoIncrement bool) string {
	// 显式 type 会绕过 MySQL 原生方言的自增类型生成，需要在整数类型上补回属性。
	mappedType := strings.TrimSpace(gormType)
	if autoIncrement && isMysqlIntegerType(mappedType) && !strings.Contains(strings.ToUpper(mappedType), "AUTO_INCREMENT") {
		return mappedType + " AUTO_INCREMENT"
	}
	return gormType
}

func isMysqlIntegerType(gormType string) bool {
	typeName := strings.ToLower(strings.TrimSpace(gormType))
	if index := strings.IndexAny(typeName, " ("); index >= 0 {
		typeName = typeName[:index]
	}

	switch typeName {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return true
	default:
		return false
	}
}

func mapToDameng(gormType string) string {
	switch strings.ToLower(gormType) {
	case "tinyint", "tinyint unsigned":
		return "tinyint"
	case "datetime":
		return "DATETIME"
	case "json":
		return "JSON"
	case "longtext", "mediumtext":
		return "CLOB"
	default:
		return gormType
	}
}
