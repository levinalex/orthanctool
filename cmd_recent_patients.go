package main

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/google/subcommands"
	"github.com/levinalex/orthanctool/api"
	"github.com/levinalex/orthanctool/patientheap"
)

const patientDetailPageSize = 200
const reverseChangeIteratorChunkSize = 1000

type recentPatientsCommand struct {
	cmdArgs             []string
	orthanc             apiFlag
	pollIntervalSeconds int
}

func RecentPatientsCommand() *recentPatientsCommand {
	return &recentPatientsCommand{}
}

func (c *recentPatientsCommand) Name() string { return "recent-patients" }
func (c *recentPatientsCommand) Usage() string {
	return c.Name() + ` --orthanc <url> [command...]:
	Iterates over all patients stored in Orthanc roughly in most recently changed order.
	Outputs JSON with patient ID and LastUpdate timestamp.
	If <command> is given, it will be run for each patient and JSON will be passed to it via stdin.` + "\n\n"
}
func (c *recentPatientsCommand) Synopsis() string {
	return "yield patient details for most recently changed patients"
}

func (c *recentPatientsCommand) SetFlags(f *flag.FlagSet) {
	f.Var(&c.orthanc, "orthanc", "Orthanc URL")
	f.IntVar(&c.pollIntervalSeconds, "poll", 60, "poll interval in seconds. Set to 0 to disable polling)")
}

func (c *recentPatientsCommand) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if c.orthanc.Api == nil {
		return fail(fmt.Errorf("orthanc URL not set"))
	}

	c.cmdArgs = f.Args()[0:]

	err := c.run(ctx, c.orthanc.Api)
	if err != nil {
		return fail(err)
	}

	return subcommands.ExitSuccess
}

// patientDetails iterates over all existing patients.
func patientDetails(ctx context.Context, source *api.Api, patients chan<- patientheap.Patient) error {
	index := 0
	for {
		details, err := source.PatientDetailsSince(ctx, index, patientDetailPageSize)
		if err != nil {
			return err
		}
		if len(details) == 0 {
			return nil
		}
		index += len(details)

		for _, d := range details {
			select {
			case patients <- patientheap.Patient{ID: d.ID, LastUpdate: d.LastUpdate}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
func watchForChanges(ctx context.Context, startIndex, stopIndex int, source *api.Api, patients chan<- patientheap.Patient, pollInterval time.Duration) error {
	return api.ChangeWatch{
		StartIndex:   startIndex,
		StopIndex:    stopIndex,
		PollInterval: pollInterval,
	}.
		Run(ctx, source, func(cng api.ChangeResult) {
			if cng.ChangeType == "StablePatient" {
				patients <- patientheap.Patient{ID: cng.ID, LastUpdate: cng.Date}
			}
		})
}

func (c *recentPatientsCommand) run(ctx context.Context, source *api.Api) error {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(ctx)
	errors := make(chan error, 0)
	patients := make(chan patientheap.Patient, 0)
	sortedPatients := patientheap.SortPatients(ctx.Done(), patients, true)
	returnError := readFirstError(errors, func() { cancel() })

	wg.Add(1)
	go func() {
		defer wg.Done()

		_, lastIndex, err := source.LastChange(ctx)
		if err != nil {
			errors <- err
			return
		}

		if c.pollIntervalSeconds > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pollInterval := time.Duration(c.pollIntervalSeconds) * time.Second
				errors <- watchForChanges(ctx, lastIndex, -1, source, patients, pollInterval)
			}()
		}

		to := lastIndex
		for to > 0 {
			from := to - reverseChangeIteratorChunkSize
			errors <- watchForChanges(ctx, from, to, source, patients, 0) // all past changes up to now
			to = from
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- patientDetails(ctx, source, patients)
		}()
	}()

	wg2 := sync.WaitGroup{}
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		for pat := range sortedPatients {
			errors <- cmdAction(c.cmdArgs, pat)
		}
	}()

	wg.Wait()
	close(patients)

	wg2.Wait()
	close(errors)

	return <-returnError
}
