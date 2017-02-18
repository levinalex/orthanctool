// Package api is a partial API client for the Orthanc DICOM server REST API.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/levinalex/go-urlutil"
)

type Api struct {
	BaseURL *url.URL
	client  *http.Client
}

func (a *Api) url(tpl string, vars map[string]string) string {
	return urlutil.MustResolveTemplate(a.BaseURL, tpl, vars).String()
}

func (a *Api) get(ctx context.Context, pathTpl string, vars map[string]string, result interface{}) error {
	req, err := http.NewRequest("GET", a.url(pathTpl, vars), nil)
	if err != nil {
		return err
	}
	_, err = a.do(ctx, req, result)
	return err
}

func (a *Api) do(ctx context.Context, req *http.Request, result interface{}) (*http.Response, error) {
	req = req.WithContext(ctx)
	resp, err := a.client.Do(req)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}
	if result != nil {
		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return nil, fmt.Errorf("json decode error: %s", err.Error())
		}
	}

	return resp, nil
}

// New returns a new API Client with default settings.
func New(baseURL string) (*Api, error) {
	u, err := url.Parse(baseURL)
	return &Api{BaseURL: u, client: &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}}, err
}
