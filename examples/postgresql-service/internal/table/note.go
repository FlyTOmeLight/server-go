package table

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/FlyTOmeLight/server-go/db"
)

type Note struct {
	ID          string       `gorm:"type:uuid;primaryKey"`
	Name        string       `gorm:"type:varchar(64);not null"`
	Description string       `gorm:"type:varchar(1024);not null"`
	Priority    int8         `gorm:"type:tinyint;not null"`
	Enabled     bool         `gorm:"not null"`
	Reviewed    sql.NullBool `gorm:"type:tinyint"`
	CreatedAt   time.Time    `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time    `gorm:"type:timestamptz;not null"`
}

func (Note) TableName() string { return "notes" }

type Store struct{ database *db.DB }

func New(database *db.DB) *Store { return &Store{database: database} }

func (s *Store) AutoMigrate(ctx context.Context) error {
	return s.database.WithContext(ctx).AutoMigrate(&Note{})
}

func (s *Store) Create(ctx context.Context, note *Note) error {
	return s.database.WithContext(ctx).Clauses(clause.Returning{}).Create(note).Error
}

func (s *Store) Get(ctx context.Context, id string) (*Note, error) {
	var note Note
	if err := s.database.WithContext(ctx).First(&note, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

func (s *Store) Update(ctx context.Context, note *Note) error {
	result := s.database.WithContext(ctx).Model(note).Clauses(clause.Returning{}).
		Select("name", "description", "priority", "enabled", "reviewed", "updated_at").Updates(note)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	result := s.database.WithContext(ctx).Delete(&Note{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) List(ctx context.Context, pageNo, pageSize int) ([]Note, int64, error) {
	query := s.database.WithContext(ctx).Model(&Note{})
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	notes := make([]Note, 0)
	err := query.Order("created_at ASC, id ASC").Offset((pageNo - 1) * pageSize).Limit(pageSize).Find(&notes).Error
	return notes, count, err
}
