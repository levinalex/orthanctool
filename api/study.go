package api

import (
	"context"
)

type GetStudyResponse struct {
	ID                   string
	IsStable             bool
	LastUpdate           string
	MainDicomTags        map[string]string
	ParentPatient        string
	PatientMainDicomTags map[string]string
	Series               []string
	Type                 string
}

func (a *Api) GetStudy(ctx context.Context, id string) (result GetStudyResponse, err error) {
	err = a.get(ctx, "studies/{id}", map[string]string{"id": id}, &result)
	return result, err
}
