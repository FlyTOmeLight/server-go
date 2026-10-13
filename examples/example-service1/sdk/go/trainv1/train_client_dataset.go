package trainv1

import (
	"bytes"
	"context"
	"net/url"
	"strconv"

	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/sdk"
)

func (client *Client) CreateDataset(ctx context.Context, req *CreateDatasetRequest) (*CreateDatasetResponse, error) {
	if req == nil {
		return nil, errors.ErrorInvalidHTTPRequest
	}
	var resp CreateDatasetResponse
	result, err := client.R().SetContext(ctx).SetBody(req).SetResult(&resp).
		Post("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets")
	if err != nil {
		return nil, err
	}
	if result.IsStatusFailure() {
		return nil, sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return &resp, nil
}

func (client *Client) ListDataset(ctx context.Context, req *ListDatasetRequest) (*ListDatasetResponse, error) {
	if req == nil {
		return nil, errors.ErrorInvalidHTTPRequest
	}
	var resp ListDatasetResponse
	result, err := client.R().SetContext(ctx).SetQueryParams(map[string]string{
		"pageNo": strconv.Itoa(req.GetPageNo()), "pageSize": strconv.Itoa(req.GetPageSize()),
		"order": req.Order, "orderBy": req.OrderBy, "filter": req.Filter,
	}).SetResult(&resp).Get("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets")
	if err != nil {
		return nil, err
	}
	if result.IsStatusFailure() {
		return nil, sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return &resp, nil
}

func (client *Client) GetDataset(ctx context.Context, req *GetDatasetRequest) (*GetDatasetResponse, error) {
	if req == nil {
		return nil, errors.ErrorInvalidHTTPRequest
	}
	var resp GetDatasetResponse
	result, err := client.R().SetContext(ctx).SetResult(&resp).
		Get("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets/" + url.PathEscape(req.LocalName))
	if err != nil {
		return nil, err
	}
	if result.IsStatusFailure() {
		return nil, sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return &resp, nil
}

func (client *Client) UpdateDataset(ctx context.Context, req *UpdateDatasetRequest) error {
	if req == nil {
		return errors.ErrorInvalidHTTPRequest
	}
	result, err := client.R().SetContext(ctx).SetBody(req).
		Put("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets/" + url.PathEscape(req.LocalName))
	if err != nil {
		return err
	}
	if result.IsStatusFailure() {
		return sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return nil
}

func (client *Client) DeleteDataset(ctx context.Context, req *DeleteDatasetRequest) error {
	if req == nil {
		return errors.ErrorInvalidHTTPRequest
	}
	result, err := client.R().SetContext(ctx).
		Delete("/v1/projects/" + url.PathEscape(req.ProjectName) + "/datasets/" + url.PathEscape(req.LocalName))
	if err != nil {
		return err
	}
	if result.IsStatusFailure() {
		return sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return nil
}
