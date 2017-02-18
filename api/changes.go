package api

import (
	"context"
	"strconv"
)

type ChangeResult struct {
	ChangeType   string
	Date         string
	ID           string
	Path         string
	ResourceType string
	Seq          int
}

type ChangesResult struct {
	Changes []ChangeResult
	Done    bool
	Last    int
}

func (a *Api) Changes(ctx context.Context, since, limit int) (result ChangesResult, err error) {
	vars := map[string]string{}
	if since > 0 {
		vars["since"] = strconv.Itoa(since)
	}
	if limit > 0 {
		vars["limit"] = strconv.Itoa(limit)
	}

	err = a.get(ctx, "changes{?last,since,limit}", vars, &result)
	return result, err
}

func (a *Api) LastChange(ctx context.Context) (result ChangeResult, last int, err error) {
	var changes ChangesResult
	err = a.get(ctx, "changes?last", nil, &changes)
	if idx := len(changes.Changes); idx > 0 {
		result = changes.Changes[idx-1]
	}
	return result, changes.Last, err
}
