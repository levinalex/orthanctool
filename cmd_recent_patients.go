package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	url                 string
	pollFutureChanges   bool
	pollIntervalSeconds int
}

func RecentPatientsCommand() *recentPatientsCommand {
	return &recentPatientsCommand{}
}

func (c *recentPatientsCommand) Name() string { return "recent-patients" }
func (c *recentPatientsCommand) Usage() string {
	return c.Name() + ` --orthanc orthanc_url [command...]:
	Iterates over all patients stored in Orthanc roughly in most recently changed order.
	Outputs JSON with patient ID and LastUpdate timestamp.
	If <command> is given, it will be run for each patient and JSON will be passed to it via stdin.` + "\n"
}
func (c *recentPatientsCommand) Synopsis() string {
	return "yield patient details for most recently changed patients"
}

func (c *recentPatientsCommand) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.url, "orthanc", "", "Orthanc URL")
	f.IntVar(&c.pollIntervalSeconds, "poll-interval", 60, "poll interval in seconds")
	f.BoolVar(&c.pollFutureChanges, "poll", true, "continuously poll for changes")
}

func (c *recentPatientsCommand) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	fail := func(e error) subcommands.ExitStatus {
		fmt.Fprintln(os.Stderr, e.Error())
		return subcommands.ExitFailure
	}

	if c.url == "" {
		f.Usage()
		return subcommands.ExitUsageError
	}
	source, err := api.New(c.url)
	if err != nil {
		return fail(err)
	}

	c.cmdArgs = f.Args()[0:]

	err = c.run(ctx, source)
	if err != nil {
		return fail(err)
	}

	return subcommands.ExitSuccess
}

// patientDetails iterates over all existing patients.
func patientDetails(done <-chan struct{}, wg *sync.WaitGroup, source *api.Api, patients chan<- patientheap.Patient, e ErrorFunc) {
	defer wg.Done()

	logger := log.New(os.Stderr, "patient-detail ", 0)

	index := 0
	for {
		logger.Printf("/patients?since=%d", index)
		details, err := source.PatientDetailsSince(index, patientDetailPageSize)
		if err != nil {
			e(err)
			return
		}
		if len(details) == 0 {
			return
		}
		index += len(details)

		for _, d := range details {
			select {
			case patients <- patientheap.Patient{ID: d.ID, LastUpdate: d.LastUpdate}:
			case <-done:
				return
			}
		}
	}
}
func watchForChanges(ctx context.Context, startIndex, stopIndex int, source *api.Api, patients chan<- patientheap.Patient, pollInterval time.Duration, e ErrorFunc) {
	err := api.ChangeWatch{
		StartIndex: startIndex, StopIndex: stopIndex,
		Logger:       log.New(os.Stderr, "", 0),
		PollInterval: pollInterval,
	}.
		Run(source, ctx, func(cng api.ChangeResult) {
			if cng.ChangeType == "StablePatient" {
				patients <- patientheap.Patient{ID: cng.ID, LastUpdate: cng.Date}
			}
		})

	if err != nil {
		e(err)
	}
}

func (c *recentPatientsCommand) cmdAction(pat patientheap.PatientOutput) error {
	cmd := c.cmdArgs

	b, err := json.Marshal(pat)
	if err != nil {
		return err
	}
	if len(cmd) == 0 {
		fmt.Println(string(b))
	} else {
		cmd := exec.Command(cmd[0], cmd[1:]...)
		cmd.Stdin = bytes.NewBuffer(b)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *recentPatientsCommand) run(ctx context.Context, source *api.Api) error {
	var err error
	wg, wg2 := sync.WaitGroup{}, sync.WaitGroup{}
	ctx, cancelFunc := context.WithCancel(ctx)
	e := func(e error) {
		cancelFunc()
		err = e
	}

	patients := make(chan patientheap.Patient, 0)
	sortedPatients := patientheap.SortPatients(ctx.Done(), patients)

	wg.Add(1)
	go patientDetails(ctx.Done(), &wg, source, patients, e)

	wg.Add(1)
	go func() {
		defer wg.Done()

		_, lastIndex, err := source.LastChange()
		if err != nil {
			e(err)
			return
		}

		if c.pollFutureChanges {
			wg.Add(1)
			go func() {
				defer wg.Done()
				watchForChanges(ctx, lastIndex, -1, source, patients,
					time.Duration(c.pollIntervalSeconds)*time.Second, e)
			}()
		}

		to := lastIndex
		for to > 0 {
			from := to - reverseChangeIteratorChunkSize
			watchForChanges(ctx, from, to, source, patients, 0, e) // all past changes up to now
			to = from
		}
	}()

	wg2.Add(1)
	go func() {
		defer wg2.Done()

		for pat := range sortedPatients {
			err := c.cmdAction(pat)
			if err != nil {
				e(err)
				break
			}
		}
	}()

	wg.Wait()

	close(patients)

	wg2.Wait()

	return err
}
