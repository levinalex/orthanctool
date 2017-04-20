package api

import (
	"context"
	"time"
)

type ChangeWatch struct {
	StartIndex   int
	StopIndex    int
	StopAtEnd    bool
	PollInterval time.Duration
}

var DefaultPollInterval = 60 * time.Second

// Run iterates through all the changes from StartIndex to StopIndex (or forever, when StopIndex is < 0)
// for all Change entries that are encountered the callback function is called.
//
// when StopIndex is -1, it will wait DefaultChangeWatchSleepTime (60 seconds) before trying again.
//
func (cw ChangeWatch) Run(ctx context.Context, api *Api, f func(ChangeResult)) error {
	sleepTime := cw.PollInterval
	if sleepTime == 0 {
		sleepTime = DefaultPollInterval
	}

	since := cw.StartIndex
	for {
		if ctx.Err() != nil {
			break
		}
		changes, err := api.Changes(ctx, since, 0)
		if err != nil {
			return err
		}
		for _, cng := range changes.Changes {
			if ctx.Err() != nil {
				return nil
			}
			f(cng)

			if cw.StopIndex > 0 && cng.Seq >= cw.StopIndex {
				return nil
			}
		}
		since = changes.Last

		if changes.Done {
			if cw.StopAtEnd {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(sleepTime):
				continue
			}
		}
	}
	return nil
}
