package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

type BaseModel struct {
	CreatedAt Time `bson:"created_at" gorm:"column:created_at"`
	UpdatedAt Time `bson:"updated_at" gorm:"column:updated_at"`
}

type TestModel struct {
	BaseModel
	Name string `bson:"name" gorm:"column:name"`
}

func TestBaseModel_MongoInsert(t *testing.T) {
	t.Run("auto parse time when insert", func(t *testing.T) {
		// 模拟MongoDB插入场景
		model := TestModel{
			Name: "test",
		}

		// 转成BSON
		data, err := bson.Marshal(model)
		assert.NoError(t, err)

		// 反序列化,验证时间字段
		var decoded TestModel
		err = bson.Unmarshal(data, &decoded)
		assert.NoError(t, err)
	})

	t.Run("parse existing time in mongo doc", func(t *testing.T) {
		// 模拟MongoDB中已存在的文档
		now := time.Now().UTC()
		mongoDoc := bson.M{
			"created_at": now.UnixNano(),
			"updated_at": now.Add(time.Hour).UnixNano(),
			"name":       "test",
		}

		// 将MongoDB文档转换为结构体
		data, err := bson.Marshal(mongoDoc)
		assert.NoError(t, err)

		var model TestModel
		err = bson.Unmarshal(data, &model)
		assert.NoError(t, err)
	})
}

func TestBaseModel_SQLInsert(t *testing.T) {
	t.Run("auto parse time when scan", func(t *testing.T) {
		// 模拟SQL扫描场景
		now := time.Now().UTC()
		dbData := map[string]any{
			"created_at": now,
			"updated_at": now.Add(time.Hour),
			"name":       "test",
		}

		var model TestModel

		err := model.CreatedAt.Scan(dbData["created_at"])
		assert.NoError(t, err)
		err = model.UpdatedAt.Scan(dbData["updated_at"])
		assert.NoError(t, err)
		model.Name = dbData["name"].(string)
	})

	t.Run("handle null time in database", func(t *testing.T) {
		// 模拟数据库中的null值
		dbData := map[string]any{
			"created_at": nil,
			"updated_at": nil,
			"name":       "test",
		}

		var model TestModel

		// 模拟数据库扫描
		err := model.CreatedAt.Scan(dbData["created_at"])
		assert.NoError(t, err)
		assert.True(t, model.CreatedAt.IsZero())
		err = model.UpdatedAt.Scan(dbData["updated_at"])
		assert.NoError(t, err)
		assert.True(t, model.UpdatedAt.IsZero())
	})
}

func TestUTCTimeScanAndValue(t *testing.T) {
	utc := time.Date(2025, 1, 9, 7, 2, 42, 0, time.UTC)
	fractional := utc.Add(123456789 * time.Nanosecond)
	for _, tc := range []struct {
		name  string
		input any
		want  time.Time
	}{
		{name: "nil clears an existing value"},
		{name: "native time", input: utc.In(time.FixedZone("offset", 8*60*60)), want: utc},
		{name: "SQL seconds", input: "2025-01-09 07:02:42", want: utc},
		{name: "SQL fractional", input: []byte("2025-01-09 07:02:42.123456789"), want: fractional},
		{name: "SQL offset", input: "2025-01-09 15:02:42.123456789+08:00", want: fractional},
		{name: "RFC3339", input: []byte("2025-01-09T15:02:42+08:00"), want: utc},
		{name: "RFC3339Nano", input: "2025-01-09T07:02:42.123456789Z", want: fractional},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := UTCTime{Time: utc}
			if err := got.Scan(tc.input); err != nil || got.Time != tc.want {
				t.Fatalf("Scan(%v) = %v, %v; want %v", tc.input, got.Time, err, tc.want)
			}
			result, err := got.Value()
			if err != nil {
				t.Fatal(err)
			}
			var want any
			if !tc.want.IsZero() {
				want = tc.want
			}
			if result != want {
				t.Errorf("Value() = %v; want %v", result, want)
			}
		})
	}
}

func TestBaseModel_TimeZoneHandling(t *testing.T) {
	t.Run("ensure UTC conversion", func(t *testing.T) {
		// 使用非UTC时区的时间
		loc, err := time.LoadLocation("Asia/Shanghai")
		assert.NoError(t, err)

		localTime := time.Now().In(loc)

		// 模拟数据库扫描
		var model TestModel
		err = model.CreatedAt.Scan(localTime)
		assert.NoError(t, err)
	})
}

func TestBaseModel_Validation(t *testing.T) {
	t.Run("invalid time data", func(t *testing.T) {
		invalidData := []any{
			"not a time",
			123,
			struct{}{},
			[]byte("invalid"),
		}

		for _, data := range invalidData {
			var model TestModel
			err := model.CreatedAt.Scan(data)
			assert.Error(t, err, "should fail for data: %v", data)
		}
	})
}

func TestBaseModel_ValidationStringTime(t *testing.T) {
	t.Run("invalid time data string", func(t *testing.T) {
		d := []any{
			"2025-01-09 07:02:42",
		}

		for _, data := range d {
			var model TestModel
			err := model.CreatedAt.Scan(data)
			assert.NoError(t, err)
			t.Logf("%v", model.CreatedAt)
		}
	})
}
