package trainv1

import (
	"net/url"
	"strconv"

	"github.com/FlyTOmeLight/server-go/errors"
)

func (client *Client) CreateDataset(req *CreateDatasetRequest) (*CreateDatasetResponse, error) {
	if req == nil {
		return nil, errors.ErrorInvalidHTTPRequest
	}
	var resp CreateDatasetResponse
	var sErr errors.Error
	result, err := client.R().SetBody(req).SetResult(&resp).SetError(&sErr).
		Post("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets")
	if err != nil {
		return nil, err
	}
	if result.IsError() {
		sErr.Status = result.StatusCode()
		return nil, &sErr
	}
	return &resp, nil
}

func (client *Client) ListDataset(req *ListDatasetRequest) (*ListDatasetResponse, error) {
	if req == nil {
		return nil, errors.ErrorInvalidHTTPRequest
	}
	var resp ListDatasetResponse
	var sErr errors.Error
	result, err := client.R().SetQueryParams(map[string]string{
		"pageNo": strconv.Itoa(req.GetPageNo()), "pageSize": strconv.Itoa(req.GetPageSize()),
		"order": req.Order, "orderBy": req.OrderBy, "filter": req.Filter,
	}).SetResult(&resp).SetError(&sErr).Get("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets")
	if err != nil {
		return nil, err
	}
	if result.IsError() {
		sErr.Status = result.StatusCode()
		return nil, &sErr
	}
	return &resp, nil
}

func (client *Client) GetDataset(req *GetDatasetRequest) (*GetDatasetResponse, error) {
	if req == nil {
		return nil, errors.ErrorInvalidHTTPRequest
	}
	var resp GetDatasetResponse
	var sErr errors.Error
	result, err := client.R().SetResult(&resp).SetError(&sErr).
		Get("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets/" + url.PathEscape(req.LocalName))
	if err != nil {
		return nil, err
	}
	if result.IsError() {
		sErr.Status = result.StatusCode()
		return nil, &sErr
	}
	return &resp, nil
}

func (client *Client) UpdateDataset(req *UpdateDatasetRequest) error {
	if req == nil {
		return errors.ErrorInvalidHTTPRequest
	}
	var sErr errors.Error
	result, err := client.R().SetBody(req).SetError(&sErr).
		Put("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets/" + url.PathEscape(req.LocalName))
	if err != nil {
		return err
	}
	if result.IsError() {
		sErr.Status = result.StatusCode()
		return &sErr
	}
	return nil
}

func (client *Client) DeleteDataset(req *DeleteDatasetRequest) error {
	if req == nil {
		return errors.ErrorInvalidHTTPRequest
	}
	var sErr errors.Error
	result, err := client.R().SetError(&sErr).
		Delete("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets/" + url.PathEscape(req.LocalName))
	if err != nil {
		return err
	}
	if result.IsError() {
		sErr.Status = result.StatusCode()
		return &sErr
	}
	return nil
}
