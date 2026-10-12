package trainv1

type TrainingAPI interface {
	CreateDataset(req *CreateDatasetRequest) (*CreateDatasetResponse, error)
	ListDataset(req *ListDatasetRequest) (*ListDatasetResponse, error)
	GetDataset(req *GetDatasetRequest) (*GetDatasetResponse, error)
	UpdateDataset(req *UpdateDatasetRequest) error
	DeleteDataset(req *DeleteDatasetRequest) error
}
