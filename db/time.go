package db

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Time wraps a SQL timestamp. Use mongodb.Time for MongoDB BSON values.
type Time struct {
	time.Time
}

func (t Time) Value() (driver.Value, error) {
	return UTCTime{Time: t.Time}.Value()
}

func (t *Time) Scan(v any) error {
	var ut UTCTime
	if err := ut.Scan(v); err != nil {
		return err
	}
	t.Time = ut.Time
	return nil
}

type UTCTime struct {
	time.Time
}

func (t UTCTime) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *UTCTime) Scan(v any) error {
	switch value := v.(type) {
	case nil:
		*t = UTCTime{}
	case time.Time:
		*t = UTCTime{Time: value.UTC()}
	case []byte:
		return t.Scan(string(value))
	case string:
		for _, layout := range []string{
			time.RFC3339Nano,
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
		} {
			if parsed, err := time.Parse(layout, value); err == nil {
				*t = UTCTime{Time: parsed.UTC()}
				return nil
			}
		}
		return fmt.Errorf("can not parse %q as a timestamp", value)
	default:
		return fmt.Errorf("can not convert %T to timestamp", v)
	}
	return nil
}
