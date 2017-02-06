package api

import (
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

func (a *Api) Changes(since, limit int) (result ChangesResult, err error) {
	vars := map[string]string{}
	if since > 0 {
		vars["since"] = strconv.Itoa(since)
	}
	if limit > 0 {
		vars["limit"] = strconv.Itoa(limit)
	}

	err = a.get("changes{?last,since,limit}", vars, &result)
	return result, err
}

func (a *Api) LastChange() (result ChangeResult, last int, err error) {
	var changes ChangesResult
	err = a.get("changes?last", nil, &changes)
	if idx := len(changes.Changes); idx > 0 {
		result = changes.Changes[idx-1]
	}
	return result, changes.Last, err
}
