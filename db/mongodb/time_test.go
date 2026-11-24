package mongodb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestTimeBSON(t *testing.T) {
	type document struct {
		CreatedAt Time `bson:"created_at"`
		UpdatedAt Time `bson:"updated_at"`
	}
	created := time.Date(2025, 1, 9, 7, 2, 42, 123456789, time.UTC)
	updated := created.Add(time.Hour)

	t.Run("round trip", func(t *testing.T) {
		original := document{CreatedAt: Time(created), UpdatedAt: Time(updated)}
		data, err := bson.Marshal(&original)
		require.NoError(t, err)
		var raw bson.M
		require.NoError(t, bson.Unmarshal(data, &raw))
		require.Equal(t, bson.M{"created_at": created.UnixNano(), "updated_at": updated.UnixNano()}, raw)
		var decoded document
		require.NoError(t, bson.Unmarshal(data, &decoded))
		require.Equal(t, original, decoded)
	})

	t.Run("read existing nanosecond timestamps", func(t *testing.T) {
		data, err := bson.Marshal(bson.M{"created_at": created.UnixNano(), "updated_at": updated.UnixNano()})
		require.NoError(t, err)
		var decoded document
		require.NoError(t, bson.Unmarshal(data, &decoded))
		require.Equal(t, created, time.Time(decoded.CreatedAt))
		require.Equal(t, updated, time.Time(decoded.UpdatedAt))
	})
}
