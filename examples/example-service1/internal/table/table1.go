package table

import (
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/sdk"
	"gorm.io/plugin/soft_delete"
)

type DatasetData struct {
	ID        string `gorm:"column:id;type:varchar(20);NOT NULL;primary_key"`
	CreatedAt db.UTCTime
	UpdatedAt db.UTCTime
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;default:0;NOT NULL"`

	Name        string `gorm:"type:varchar(64);NOT NULL"`
	ProjectName string `gorm:"type:varchar(64);NOT NULL"`
	LocalName   string `gorm:"type:varchar(64);NOT NULL"`

	Description *string `gorm:"type:varchar(128);NOT NULL"`
	DatasetType string  `gorm:"type:varchar(128);NOT NULL"`
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

func AutoMigrateDataset() {
	_ = db.DefaultDB().AutoMigrate(&DatasetData{})
}

func CreateDataset(dataset *DatasetData) error {
	err := db.DefaultDB().Create(dataset).Error
	return errors.NewDBError(err)
}

func UpdateDataset(dataset *DatasetData) error {
	err := db.DefaultDB().Where(&DatasetFilter{Name: dataset.Name}).Updates(dataset).Error
	return errors.NewDBError(err)
}

func GetDataset(name string) (*DatasetData, error) {
	var t DatasetData
	err := db.DefaultDB().Where(&DatasetFilter{Name: name}).Find(&t).Error
	if err != nil {
		return nil, errors.NewDBError(err)
	}
	return &t, nil
}

func ListDataset(filters DatasetFilter, paging sdk.PagingRequest) ([]*DatasetData, error) {
	sql := db.DefaultDB().
		Where(filters).
		Offset(paging.Offset()).
		Limit(paging.Limit()).
		Order(paging.DBOrder())

	if filters.Filter != "" {
		sql.Where("local_name LIKE ? or description LIKE ?", "%"+filters.Filter+"%", "%"+filters.Filter+"%")
	}

	data := make([]*DatasetData, 0)
	err := sql.Find(&data).Error
	if err != nil {
		return nil, errors.NewDBError(err)
	}
	return data, nil
}

func DeleteDataset(name string) error {
	err := db.DefaultDB().Where(&DatasetFilter{Name: name}).Delete(&DatasetData{}).Error
	return errors.NewDBError(err)
}

func CountDataset(filters DatasetFilter) (int, error) {
	var count int64
	sql := db.DefaultDB().Where(filters)
	if filters.Filter != "" {
		sql.Where("local_name LIKE ? or description LIKE ?", "%"+filters.Filter+"%", "%"+filters.Filter+"%")
	}
	err := sql.Model(&DatasetData{}).Count(&count).Error
	return int(count), errors.NewDBError(err)
}
