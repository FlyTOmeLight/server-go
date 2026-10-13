package helloworldv1

import "context"

type HelloWorldAPI interface {
	CreateModule(ctx context.Context, req *CreateModuleRequest) (*CreateModuleResponse, error)
	ListModule(ctx context.Context, req *ListModuleRequest) (*ListModuleResponse, error)
	GetModule(ctx context.Context, req *GetModuleRequest) (*GetModuleResponse, error)
	UpdateModule(ctx context.Context, req *UpdateModuleRequest) error
	DeleteModule(ctx context.Context, req *DeleteModuleRequest) error
}
