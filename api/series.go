package api

import (
	"context"
)

type GetSeriesResponse struct {
	ID            string
	IsStable      bool
	Instances     []string
	LastUpdate    string
	MainDicomTags map[string]string
	ParentStudy   string
	Status        string
	Type          string
}

func (a *Api) GetSeries(ctx context.Context, id string) (result GetSeriesResponse, err error) {
	err = a.get(ctx, "series/{id}", map[string]string{"id": id}, &result)
	return result, err
}
