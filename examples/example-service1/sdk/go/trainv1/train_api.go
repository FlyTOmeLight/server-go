package trainv1

import "context"

type TrainingAPI interface {
	CreateDataset(ctx context.Context, req *CreateDatasetRequest) (*CreateDatasetResponse, error)
	ListDataset(ctx context.Context, req *ListDatasetRequest) (*ListDatasetResponse, error)
	GetDataset(ctx context.Context, req *GetDatasetRequest) (*GetDatasetResponse, error)
	UpdateDataset(ctx context.Context, req *UpdateDatasetRequest) error
	DeleteDataset(ctx context.Context, req *DeleteDatasetRequest) error
}
