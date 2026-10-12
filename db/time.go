package db

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson/bsontype"

	"github.com/FlyTOmeLight/server-go/db/mongodb"
)

type Time struct {
	time.Time
}

func (t Time) MarshalBSONValue() (bsontype.Type, []byte, error) {
	mt := mongodb.Time(t.Time)
	return (&mt).MarshalBSONValue()
}

func (t *Time) UnmarshalBSONValue(bt bsontype.Type, data []byte) error {
	var mt mongodb.Time
	if err := (&mt).UnmarshalBSONValue(bt, data); err != nil {
		return err
	}
	t.Time = time.Time(mt)
	return nil
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
	var zeroTime time.Time
	if t.Time.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *UTCTime) Scan(v any) error {
	vType := reflect.TypeOf(v)
	if vType == nil {
		return fmt.Errorf("can not convert NULL to timestamp")
	}
	// 适配SQLite
	if vType.Kind() == reflect.String {
		tm, err := time.Parse("2006-01-02 15:04:05", v.(string))
		if err != nil {
			return err
		}
		*t = UTCTime{Time: tm.UTC()}
		return nil
	}
	if value, ok := v.(time.Time); ok {
		*t = UTCTime{Time: value.UTC()}
		return nil
	}
	return fmt.Errorf("can not convert %v to timestamp", v)
}
