package helloworldv1

type HelloWorldAPI interface {
	CreateModule(req *CreateModuleRequest) (*CreateModuleResponse, error)
	ListModule(req *ListModuleRequest) (*ListModuleResponse, error)
	GetModule(req *GetModuleRequest) (*GetModuleResponse, error)
	UpdateModule(req *UpdateModuleRequest) error
	DeleteModule(req *DeleteModuleRequest) error
}
