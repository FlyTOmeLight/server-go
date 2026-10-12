package mongodb

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

// Time is alias type for time.Time
type Time time.Time

// MarshalBSONValue mongodb是存储bson格式，因此需要实现序列化bsonvalue(这里不能实现MarshalBSON，MarshalBSON是处理Document的)，将时间转换成ns时间戳(int64)
func (t *Time) MarshalBSONValue() (bsontype.Type, []byte, error) {
	nano := time.Time(*t).UnixNano()
	return bson.MarshalValue(nano)
}

// UnmarshalBSONValue 实现bson反序列化，从mongodb中读取数据转换成time.Time格式，这里用到了bsoncore中的方法读取数据转换成int64然后再转换成time.Time
func (t *Time) UnmarshalBSONValue(bt bsontype.Type, data []byte) error {
	v, _, valid := bsoncore.ReadValue(data, bt)
	if !valid {
		return fmt.Errorf("%s, %s, %s", "UnmarshalBSONValue error:", bt, data)
	}

	if v.Type == bson.TypeInt64 {
		*t = Time(time.Unix(0, v.Int64()).UTC())
	}
	return nil
}
