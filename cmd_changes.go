package main

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/google/subcommands"
	"github.com/levinalex/orthanctool/api"
)

type changesCommand struct {
	cmdArgs             []string
	orthanc             apiFlag
	pollFutureChanges   bool
	allChanges          bool
	filter              string
	pollIntervalSeconds int
}

func ChangesCommand() *changesCommand {
	return &changesCommand{}
}

func (c changesCommand) Name() string { return "changes" }
func (c changesCommand) Usage() string {
	return c.Name() + ` --orthanc <url> [--all] [--poll] [command...]:
	Iterates over changes in Orthanc.
	Outputs each change as JSON. 
	If command is given, it will be run for each change and JSON will be passed to it via stdin.` + "\n\n"
}
func (c changesCommand) Synopsis() string { return "yield change entries" }

func (c *changesCommand) SetFlags(f *flag.FlagSet) {
	f.Var(&c.orthanc, "orthanc", "Orthanc URL")
	f.IntVar(&c.pollIntervalSeconds, "poll-interval", 60, "poll interval in seconds")
	f.BoolVar(&c.pollFutureChanges, "poll", true, "continuously poll for changes")
	f.BoolVar(&c.allChanges, "all", true, "yield past changes")
	f.StringVar(&c.filter, "filter", "", "only output changes of this type")
}

func (c *changesCommand) run(ctx context.Context) error {
	wg := sync.WaitGroup{}
	errors := make(chan error, 0)
	returnError := readFirstError(errors, func() {})

	_, lastIndex, err := c.orthanc.Api.LastChange(ctx)
	if err != nil {
		errors <- err
	}

	onChange := func(cng api.ChangeResult) {
		cmdAction(c.cmdArgs, cng)
	}

	if c.pollFutureChanges {
		wg.Add(1)
		go func() {
			defer wg.Wait()

			pollInterval := time.Duration(c.pollIntervalSeconds) * time.Second
			errors <- api.ChangeWatch{
				StartIndex:   lastIndex,
				PollInterval: pollInterval,
			}.Run(ctx, c.orthanc.Api, onChange)
		}()
	}

	if c.allChanges {
		wg.Add(1)
		go func() {
			defer wg.Done()

			errors <- api.ChangeWatch{
				StartIndex: 0,
				StopIndex:  lastIndex,
			}.Run(ctx, c.orthanc.Api, onChange)

		}()
	}

	wg.Wait()

	close(errors)
	return <-returnError
}

func (c *changesCommand) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	c.cmdArgs = f.Args()[0:]
	err := c.run(ctx)

	if err != nil {
		return fail(err)
	}
	return subcommands.ExitSuccess
}
