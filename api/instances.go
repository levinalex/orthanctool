package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
)

func (a *Api) Instances(ctx context.Context, since int, limit int) (result []string, err error) {
	vars := map[string]string{}
	if since > 0 || limit > 0 {
		vars["since"] = strconv.Itoa(since)
		vars["limit"] = strconv.Itoa(limit)
	}
	err = a.get(ctx, "instances{?since,limit}", vars, &result)

	return result, err
}

type GetInstanceResponse struct {
	ID            string            `json:"ID"`
	Type          string            `json:"Type"`
	FileSize      int               `json:"FileSize"`
	MainDicomTags map[string]string `json:"MainDicomTags"`
}

func (a *Api) GetInstance(ctx context.Context, id string) (result GetInstanceResponse, err error) {
	err = a.get(ctx, "instances/{id}", map[string]string{"id": id}, &result)
	return result, err
}

type InstanceTag struct {
	Name  string      `json:"Name"`
	Type  string      `json:"Type"`
	Value interface{} `json:"Value"`
}

type GetInstanceTagsResponse map[string]InstanceTag

func (a *Api) GetInstanceTags(ctx context.Context, id string) (result GetInstanceTagsResponse, err error) {
	result = make(GetInstanceTagsResponse)
	err = a.get(ctx, "instances/{id}/tags", map[string]string{"id": id}, &result)
	return result, err
}

func (a *Api) InstanceFile(ctx context.Context, id string) (r io.ReadCloser, len int64, err error) {
	req, err := http.NewRequest("GET", a.url("instances/{id}/file", map[string]string{"id": id}), nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := a.do(ctx, req, nil)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.ContentLength, nil
}

func (a *Api) GetInstancePreview(ctx context.Context, id string) (r io.ReadCloser, len int64, err error) {
	req, err := http.NewRequest("GET", a.url("instances/{id}/preview", map[string]string{"id": id}), nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := a.do(ctx, req, nil)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.ContentLength, nil
}

type PostInstanceResponse struct {
	ID     string `json:"ID"`
	Path   string `json:"Path"`
	Status string `json:"Status"`
}

func (a *Api) PostInstance(ctx context.Context, data io.Reader, len int64) (result PostInstanceResponse, err error) {
	url := a.url("instances", nil)

	req, err := http.NewRequest("POST", url, data)
	if err != nil {
		return result, err
	}
	req.ContentLength = len

	_, err = a.do(ctx, req, &result)
	return result, err
}
