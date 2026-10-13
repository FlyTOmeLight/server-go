package db

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var errBooleanValue = errors.New("boolean value conversion failed")

type rejectedBoolean bool

func (rejectedBoolean) Value() (driver.Value, error) { return nil, errBooleanValue }

func TestTinyIntPreservesFieldSemantics(t *testing.T) {
	type row struct {
		Priority        int8         `gorm:"type:tinyint"`
		Count           uint8        `gorm:"type:tinyint unsigned"`
		Enabled         bool         `gorm:"type:tinyint"`
		Optional        *bool        `gorm:"type:tinyint unsigned"`
		Nullable        sql.NullBool `gorm:"type:tinyint"`
		Native          bool
		Explicit        bool          `gorm:"type:boolean"`
		Aliased         *bool         `gorm:"type:bool"`
		NullablePointer *sql.NullBool `gorm:"type:tinyint"`
	}
	parsed, err := schema.Parse(&row{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name           string
		dbType         string
		integerBoolean bool
		booleanType    string
	}{
		{name: "PostgreSQL", dbType: "postgres", booleanType: "BOOLEAN"},
		{name: "GaussDB", dbType: "gaussdb", integerBoolean: true, booleanType: "SMALLINT"},
		{name: "Kingbase", dbType: "postgres", integerBoolean: true, booleanType: "SMALLINT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dialect := NewAIBaseDialector(test.dbType, postgres.New(postgres.Config{}))
			dialect.integerBoolean = test.integerBoolean
			for _, field := range parsed.Fields {
				want := "SMALLINT"
				if field.Name != "Priority" && field.Name != "Count" {
					want = test.booleanType
				}
				if field.Name == "Aliased" && !test.integerBoolean {
					want = "bool" // PostgreSQL accepts the explicit bool alias unchanged.
				}
				if got := dialect.DataTypeOf(field); !strings.EqualFold(got, want) {
					t.Errorf("%s (%s): got %s, want %s", field.Name, field.FieldType, got, want)
				}
			}
			database, err := gorm.Open(dialect, &gorm.Config{
				DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			connection, err := database.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = connection.Close() })
			for _, enabled := range []bool{false, true} {
				nullable := sql.NullBool{Bool: enabled, Valid: true}
				statement := database.Create(
					&row{
						Priority: 2, Count: 255, Enabled: enabled, Optional: &enabled,
						Nullable: nullable, NullablePointer: &nullable,
						Native: enabled, Explicit: enabled, Aliased: &enabled,
					},
				).Statement
				want := []any{
					int64(2),
					int64(255),
					enabled,
					enabled,
					enabled,
					enabled,
					enabled,
					enabled,
					enabled,
				}
				if test.integerBoolean {
					integer := int64(0)
					if enabled {
						integer = 1
					}
					for index := 2; index < len(want); index++ {
						want[index] = integer
					}
				}
				bound := make([]any, len(statement.Vars))
				for index, value := range statement.Vars {
					bound[index], err = driver.DefaultParameterConverter.ConvertValue(value)
					if err != nil {
						t.Fatal(err)
					}
				}
				if !reflect.DeepEqual(bound, want) {
					t.Errorf("driver values: got %#v, want %#v", bound, want)
				}
			}
			statement := database.Create(&row{}).Statement
			for _, index := range []int{3, 4, 7, 8} {
				value, err := driver.DefaultParameterConverter.ConvertValue(statement.Vars[index])
				if err != nil || value != nil {
					t.Errorf("NULL value %d changed: value=%v error=%v", index, value, err)
				}
			}
			statement = database.Create(&struct{ Value rejectedBoolean }{Value: true}).Statement
			if _, err := driver.DefaultParameterConverter.ConvertValue(
				statement.Vars[0],
			); !errors.Is(
				err,
				errBooleanValue,
			) {
				t.Fatalf("driver value error lost: %v", err)
			}
		})
	}
}
