// implement according to https://gorm.io/docs/data_types.html, borrow idea from gorm.io/datatypes
package datatypes

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringMap defined type map[string]string, need to implements driver.Valuer, sql.Scanner interface
type StringMap map[string]string

// Value return map value
func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan scan value into map
func (m *StringMap) Scan(value any) error {
	var b []byte
	switch value := value.(type) {
	case nil:
	case []byte:
		b = value
	case string:
		b = []byte(value)
	default:
		return fmt.Errorf("cannot scan %T into StringMap", value)
	}
	if len(b) == 0 {
		*m = nil
		return nil
	}

	var parsed StringMap
	if err := json.Unmarshal(b, &parsed); err != nil {
		return err
	}
	*m = parsed
	return nil
}

// GormDataType declares the default database type for StringMap.
func (StringMap) GormDataType() string {
	return "text"
}
