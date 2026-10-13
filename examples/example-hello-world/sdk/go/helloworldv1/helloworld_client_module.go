package helloworldv1

import "context"

func (c *Client) CreateModule(ctx context.Context, req *CreateModuleRequest) (*CreateModuleResponse, error) {
	var resp CreateModuleResponse

	result, err := c.R().SetContext(ctx).SetBody(req).SetResult(&resp).Post("/v1/CreateModule")
	if err != nil {
		return nil, err
	}
	if err := responseError(result.StatusCode(), result.Bytes()); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListModule(ctx context.Context, req *ListModuleRequest) (*ListModuleResponse, error) {
	var resp ListModuleResponse

	result, err := c.R().SetContext(ctx).SetBody(req).SetResult(&resp).Post("/v1/ListModule")
	if err != nil {
		return nil, err
	}
	if err := responseError(result.StatusCode(), result.Bytes()); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetModule(ctx context.Context, req *GetModuleRequest) (*GetModuleResponse, error) {
	var resp GetModuleResponse

	result, err := c.R().SetContext(ctx).SetBody(req).SetResult(&resp).Post("/v1/GetModule")
	if err != nil {
		return nil, err
	}
	if err := responseError(result.StatusCode(), result.Bytes()); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateModule(ctx context.Context, req *UpdateModuleRequest) error {
	result, err := c.R().SetContext(ctx).SetBody(req).Post("/v1/UpdateModule")
	if err != nil {
		return err
	}
	return responseError(result.StatusCode(), result.Bytes())
}

func (c *Client) DeleteModule(ctx context.Context, req *DeleteModuleRequest) error {
	result, err := c.R().SetContext(ctx).SetBody(req).Post("/v1/DeleteModule")
	if err != nil {
		return err
	}
	return responseError(result.StatusCode(), result.Bytes())
}
