package table

import (
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/sdk"
)

type ModuleData struct {
	db.BaseData

	// 业务字段可添加在下面
}

// TableName 定义表名
func (t *ModuleData) TableName() string {
	return "module"
}

type ModuleFilter struct {
	ID        string `gorm:"column:id"`
	Name      string
	ProjectID string `gorm:"column:project_id"`
	Filter    string `gorm:"-"`
}

func (f ModuleFilter) BuildCondition(tx *db.DB) *db.DB {
	sql := tx.Where(f)
	if f.Filter != "" {
		sql.Where("id LIKE ? or name LIKE ?", "%"+f.Filter+"%", "%"+f.Filter+"%")
	}
	return sql
}

func AutoMigrateModule() {
	_ = db.DefaultDB().AutoMigrate(&ModuleData{})
}

func CreateModule(example *ModuleData, tx *db.DB) error {
	if tx == nil {
		tx = db.DefaultDB()
	}
	err := tx.Create(example).Error
	return errors.NewDBError(err)
}

func ListModule(filter ModuleFilter, paging sdk.PagingRequest) ([]*ModuleData, error) {
	sql := db.DefaultDB().Offset(paging.Offset()).Limit(paging.Limit()).Order(paging.DBOrder())
	sql = filter.BuildCondition(sql)

	data := make([]*ModuleData, 0)
	err := sql.Find(&data).Error
	if err != nil {
		return nil, errors.NewDBError(err)
	}
	return data, nil
}

func CountModule(filter ModuleFilter) (int, error) {
	var count int64
	sql := filter.BuildCondition(db.DefaultDB())
	err := sql.Model(&ModuleData{}).Count(&count).Error
	return int(count), errors.NewDBError(err)
}

func GetModule(filter ModuleFilter) (*ModuleData, error) {
	var data ModuleData
	err := db.DefaultDB().Where(filter).Take(&data).Error
	if err != nil {
		return nil, errors.NewDBError(err)
	}
	return &data, nil
}

func UpdateModule(data *ModuleData, tx *db.DB) error {
	if tx == nil {
		tx = db.DefaultDB()
	}
	err := tx.Where(&ModuleFilter{ID: data.ID, Name: data.Name}).Updates(data).Error
	return errors.NewDBError(err)
}

func DeleteModule(filter ModuleFilter, tx *db.DB) error {
	if tx == nil {
		tx = db.DefaultDB()
	}
	err := db.DefaultDB().Where(filter).Delete(&ModuleData{}).Error
	return errors.NewDBError(err)
}
