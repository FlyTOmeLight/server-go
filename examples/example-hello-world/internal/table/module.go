package table

import (
	"context"

	"github.com/FlyTOmeLight/server-go/db"
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
		sql = sql.Where("(id LIKE ? OR name LIKE ?)", "%"+f.Filter+"%", "%"+f.Filter+"%")
	}
	return sql
}

func AutoMigrateModule(ctx context.Context, database *db.DB) error {
	return database.WithContext(ctx).AutoMigrate(&ModuleData{})
}

func CreateModule(ctx context.Context, tx *db.DB, example *ModuleData) error {
	tx = tx.WithContext(ctx)
	err := tx.Create(example).Error
	return db.NewError(err)
}

func ListModule(
	ctx context.Context,
	database *db.DB,
	filter ModuleFilter,
	paging sdk.PagingRequest,
) ([]*ModuleData, error) {
	sql := database.WithContext(ctx).Offset(paging.Offset()).Limit(paging.Limit()).Order(paging.DBOrder())
	sql = filter.BuildCondition(sql)

	data := make([]*ModuleData, 0)
	err := sql.Find(&data).Error
	if err != nil {
		return nil, db.NewError(err)
	}
	return data, nil
}

func CountModule(ctx context.Context, database *db.DB, filter ModuleFilter) (int, error) {
	var count int64
	sql := filter.BuildCondition(database.WithContext(ctx))
	err := sql.Model(&ModuleData{}).Count(&count).Error
	return int(count), db.NewError(err)
}

func GetModule(ctx context.Context, database *db.DB, filter ModuleFilter) (*ModuleData, error) {
	var data ModuleData
	err := database.WithContext(ctx).Where(filter).Take(&data).Error
	if err != nil {
		return nil, db.NewError(err)
	}
	return &data, nil
}

func UpdateModule(ctx context.Context, tx *db.DB, data *ModuleData) error {
	tx = tx.WithContext(ctx)
	err := tx.Where(&ModuleFilter{ID: data.ID, Name: data.Name}).Updates(data).Error
	return db.NewError(err)
}

func DeleteModule(ctx context.Context, tx *db.DB, filter ModuleFilter) error {
	err := tx.WithContext(ctx).Where(filter).Delete(&ModuleData{}).Error
	return db.NewError(err)
}
