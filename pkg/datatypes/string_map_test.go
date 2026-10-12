package datatypes

import (
	"database/sql/driver"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

type UserWithStringMap struct {
	gorm.Model
	Name       string
	Attributes StringMap `gorm:"type:text"`
}

func TestMain(m *testing.M) {
	dbName := "sm.db"
	_ = os.Remove(dbName)
	var err error
	if DB, err = gorm.Open(sqlite.Open(dbName), &gorm.Config{}); err != nil {
		log.Fatalf("connect database failed %v", err)
	}
	if err := DB.Migrator().AutoMigrate(&UserWithStringMap{}); err != nil {
		log.Fatalf("failed to migrate, got error: %v", err)
	}

	os.Exit(m.Run())
}

var _ driver.Valuer = &StringMap{}

func TestStringMap(t *testing.T) {
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
			if err := DB.Create(&tc.user).Error; err != nil {
				t.Fatalf("Failed to create users %v", err)
			}

			var result UserWithStringMap
			if err := DB.Where("name = ?", tc.user.Name).First(&result).Error; err != nil {
				t.Fatalf("failed to find user, got error %v", err)
			}

			assert.Equal(t, tc.user.Attributes, result.Attributes)
		})
	}
}
