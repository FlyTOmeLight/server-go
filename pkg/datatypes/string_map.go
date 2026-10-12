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
func (m *StringMap) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value: %v is not []byte", value)
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, &m)
}
