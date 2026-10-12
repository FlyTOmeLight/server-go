package table

import (
	"context"
	"errors"

	"github.com/FlyTOmeLight/server-go/db"
)

func AutoMigrate(ctx context.Context, database *db.DB) error {
	return errors.Join(
		AutoMigrateModule(ctx, database),
	)
}
