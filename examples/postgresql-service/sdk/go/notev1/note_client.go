package notev1

import (
	"bytes"
	"context"
	"net/url"
	"strconv"

	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/sdk"
)

type Client struct{ *sdk.Client }

func NewClient(endpoint string) (*Client, error) {
	client, err := sdk.NewClientWithConfig(&sdk.Config{Endpoint: endpoint, Timeout: 10})
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

func (c *Client) CreateNote(ctx context.Context, req *CreateNoteRequest) (*Note, error) {
	if req == nil {
		return nil, serrors.ErrorInvalidHTTPRequest
	}
	var note Note
	result, err := c.R().SetContext(ctx).SetBody(req).SetResult(&note).Post("/v1/notes")
	if err != nil {
		return nil, err
	}
	if result.IsStatusFailure() {
		return nil, sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return &note, nil
}

func (c *Client) GetNote(ctx context.Context, req *GetNoteRequest) (*Note, error) {
	if req == nil {
		return nil, serrors.ErrorInvalidHTTPRequest
	}
	var note Note
	result, err := c.R().SetContext(ctx).SetResult(&note).Get("/v1/notes/" + url.PathEscape(req.ID))
	if err != nil {
		return nil, err
	}
	if result.IsStatusFailure() {
		return nil, sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return &note, nil
}

func (c *Client) UpdateNote(ctx context.Context, req *UpdateNoteRequest) (*Note, error) {
	if req == nil {
		return nil, serrors.ErrorInvalidHTTPRequest
	}
	var note Note
	result, err := c.R().SetContext(ctx).SetBody(req).SetResult(&note).Put("/v1/notes/" + url.PathEscape(req.ID))
	if err != nil {
		return nil, err
	}
	if result.IsStatusFailure() {
		return nil, sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return &note, nil
}

func (c *Client) DeleteNote(ctx context.Context, req *DeleteNoteRequest) error {
	if req == nil {
		return serrors.ErrorInvalidHTTPRequest
	}
	result, err := c.R().SetContext(ctx).Delete("/v1/notes/" + url.PathEscape(req.ID))
	if err != nil {
		return err
	}
	if result.IsStatusFailure() {
		return sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return nil
}

func (c *Client) ListNotes(ctx context.Context, req *ListNotesRequest) (*ListNotesResponse, error) {
	if req == nil {
		return nil, serrors.ErrorInvalidHTTPRequest
	}
	query := map[string]string{}
	if req.PageNo != 0 {
		query["pageNo"] = strconv.Itoa(req.PageNo)
	}
	if req.PageSize != 0 {
		query["pageSize"] = strconv.Itoa(req.PageSize)
	}
	var notes ListNotesResponse
	result, err := c.R().SetContext(ctx).SetQueryParams(query).SetResult(&notes).Get("/v1/notes")
	if err != nil {
		return nil, err
	}
	if result.IsStatusFailure() {
		return nil, sdk.DecodeError(result.StatusCode(), bytes.NewReader(result.Bytes()))
	}
	return &notes, nil
}
