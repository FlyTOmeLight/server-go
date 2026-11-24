package datatypes

import (
	"database/sql/driver"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type UserWithStringMap struct {
	gorm.Model
	Name       string
	Attributes StringMap
}

var _ driver.Valuer = &StringMap{}

func TestStringMap(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sm.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&UserWithStringMap{}))

	testCases := []struct {
		desc string
		user UserWithStringMap
	}{
		{
			desc: "nil attributes",
			user: UserWithStringMap{
				Name:       "1",
				Attributes: nil,
			},
		},
		{
			desc: "normal attributes",
			user: UserWithStringMap{
				Name:       "2",
				Attributes: StringMap{"age": "18"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if err := db.Create(&tc.user).Error; err != nil {
				t.Fatalf("Failed to create users %v", err)
			}

			var result UserWithStringMap
			if err := db.Where("name = ?", tc.user.Name).First(&result).Error; err != nil {
				t.Fatalf("failed to find user, got error %v", err)
			}

			assert.Equal(t, tc.user.Attributes, result.Attributes)
		})
	}
}

func TestStringMapScan(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   any
		want    StringMap
		wantErr bool
	}{
		{name: "string replaces contents", input: `{"new":"value"}`, want: StringMap{"new": "value"}},
		{name: "bytes replace contents", input: []byte(`{"new":"value"}`), want: StringMap{"new": "value"}},
		{name: "empty object", input: `{}`, want: StringMap{}},
		{name: "nil"},
		{name: "empty bytes", input: []byte{}},
		{name: "empty string", input: ""},
		{name: "JSON null", input: "null"},
		{name: "unsupported type", input: true, want: StringMap{"old": "value"}, wantErr: true},
		{name: "invalid map value", input: `{"new":"value","number":1}`, want: StringMap{"old": "value"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := StringMap{"old": "value"}
			err := got.Scan(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
