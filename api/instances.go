package api

import (
	"io"
	"net/http"
	"strconv"
)

func (a *Api) Instances(since int, limit int) (result []string, err error) {
	vars := map[string]string{}
	if since > 0 || limit > 0 {
		vars["since"] = strconv.Itoa(since)
		vars["limit"] = strconv.Itoa(limit)
	}
	err = a.get("instances{?since,limit}", vars, &result)

	return result, err
}

func (a *Api) InstanceFile(id string) (r io.ReadCloser, len int64, err error) {
	req, err := http.NewRequest("GET", a.url("instances/{id}/file", map[string]string{"id": id}), nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := a.do(req, nil)
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

func (a *Api) PostInstance(data io.Reader, len int64) (result PostInstanceResponse, err error) {
	url := a.url("instances", nil)

	req, err := http.NewRequest("POST", url, data)
	if err != nil {
		return result, err
	}
	req.ContentLength = len

	_, err = a.do(req, &result)
	return result, err
}
