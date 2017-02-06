package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/levinalex/go-urlutil"
)

type Api struct {
	baseURL *url.URL
	client  *http.Client
}

func (a *Api) url(tpl string, vars map[string]string) string {
	return urlutil.MustResolveTemplate(a.baseURL, tpl, vars).String()
}

func (a *Api) get(pathTpl string, vars map[string]string, result interface{}) error {
	req, err := http.NewRequest("GET", a.url(pathTpl, vars), nil)
	if err != nil {
		return err
	}
	_, err = a.do(req, result)
	return err
}

func (a *Api) do(req *http.Request, result interface{}) (*http.Response, error) {
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

func New(s string) (*Api, error) {
	u, err := url.Parse(s)
	return &Api{baseURL: u, client: &http.Client{
		Timeout: 600 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}}, err
}
