package helloworldv1

import "github.com/FlyTOmeLight/server-go/errors"

func (c *Client) CreateModule(req *CreateModuleRequest) (*CreateModuleResponse, error) {
	var resp CreateModuleResponse
	var sErr errors.Error

	result, err := c.R().SetBody(req).SetResult(&resp).SetError(&sErr).Post("/v1/CreateModule")
	if err != nil {
		return nil, err
	}
	if result.IsError() {
		return nil, &sErr
	}
	return &resp, nil
}

func (c *Client) ListModule(req *ListModuleRequest) (*ListModuleResponse, error) {
	var resp ListModuleResponse
	var sErr errors.Error

	result, err := c.R().SetBody(req).SetResult(&resp).SetError(&sErr).Post("/v1/ListModule")
	if err != nil {
		return nil, err
	}
	if result.IsError() {
		return nil, &sErr
	}
	return &resp, nil
}

func (c *Client) GetModule(req *GetModuleRequest) (*GetModuleResponse, error) {
	var resp GetModuleResponse
	var sErr errors.Error

	result, err := c.R().SetBody(req).SetResult(&resp).SetError(&sErr).Post("/v1/GetModule")
	if err != nil {
		return nil, err
	}
	if result.IsError() {
		return nil, &sErr
	}
	return &resp, nil
}

func (c *Client) UpdateModule(req *UpdateModuleRequest) error {
	var sErr errors.Error
	result, err := c.R().SetBody(req).SetError(&sErr).Post("/v1/UpdateModule")
	if err != nil {
		return err
	}
	if result.IsError() {
		return &sErr
	}
	return nil
}

func (c *Client) DeleteModule(req *DeleteModuleRequest) error {
	var sErr errors.Error
	result, err := c.R().SetBody(req).Post("/v1/DeleteModule")
	if err != nil {
		return err
	}
	if result.IsError() {
		return &sErr
	}
	return nil
}
