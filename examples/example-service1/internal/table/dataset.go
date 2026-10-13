package table

import (
	"context"

	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/sdk"
	"gorm.io/plugin/soft_delete"

	train "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
)

type DatasetData struct {
	ID        string `gorm:"column:id;type:varchar(20);NOT NULL;primary_key"`
	CreatedAt db.UTCTime
	UpdatedAt db.UTCTime
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;default:0;NOT NULL"`

	Name        string `gorm:"type:varchar(64);NOT NULL"`
	ProjectName string `gorm:"type:varchar(64);NOT NULL"`
	LocalName   string `gorm:"type:varchar(64);NOT NULL"`

	Description *string          `gorm:"type:varchar(128);NOT NULL"`
	FileFormat  train.FileFormat `gorm:"type:varchar(64);NOT NULL"`
}

type DatasetFilter struct {
	Name        string
	ProjectName string
	Filter      string `gorm:"-"`
}

// TableName 定义表名
func (t *DatasetData) TableName() string {
	return "dataset"
}

type Store struct{ database *db.DB }

func New(database *db.DB) *Store { return &Store{database: database} }

func (s *Store) AutoMigrate(ctx context.Context) error {
	return s.database.WithContext(ctx).AutoMigrate(&DatasetData{})
}

func (s *Store) CreateDataset(ctx context.Context, dataset *DatasetData) error {
	err := s.database.WithContext(ctx).Create(dataset).Error
	return db.NewError(err)
}

func (s *Store) UpdateDataset(ctx context.Context, dataset *DatasetData) error {
	err := s.database.WithContext(ctx).Where(&DatasetFilter{Name: dataset.Name}).Updates(dataset).Error
	return db.NewError(err)
}

func (s *Store) GetDataset(ctx context.Context, name string) (*DatasetData, error) {
	var t DatasetData
	err := s.database.WithContext(ctx).Where(&DatasetFilter{Name: name}).First(&t).Error
	if err != nil {
		return nil, db.NewError(err)
	}
	return &t, nil
}

func (s *Store) ListDataset(
	ctx context.Context,
	filters DatasetFilter,
	paging sdk.PagingRequest,
) ([]*DatasetData, error) {
	sql := s.database.WithContext(ctx).
		Where(filters).
		Offset(paging.Offset()).
		Limit(paging.Limit()).
		Order(paging.DBOrder())

	if filters.Filter != "" {
		sql = sql.Where(
			"(local_name LIKE ? OR description LIKE ?)",
			"%"+filters.Filter+"%",
			"%"+filters.Filter+"%",
		)
	}

	data := make([]*DatasetData, 0)
	err := sql.Find(&data).Error
	if err != nil {
		return nil, db.NewError(err)
	}
	return data, nil
}

func (s *Store) DeleteDataset(ctx context.Context, name string) error {
	err := s.database.WithContext(ctx).Where(&DatasetFilter{Name: name}).Delete(&DatasetData{}).Error
	return db.NewError(err)
}

func (s *Store) CountDataset(ctx context.Context, filters DatasetFilter) (int, error) {
	var count int64
	sql := s.database.WithContext(ctx).Where(filters)
	if filters.Filter != "" {
		sql = sql.Where(
			"(local_name LIKE ? OR description LIKE ?)",
			"%"+filters.Filter+"%",
			"%"+filters.Filter+"%",
		)
	}
	err := sql.Model(&DatasetData{}).Count(&count).Error
	return int(count), db.NewError(err)
}
