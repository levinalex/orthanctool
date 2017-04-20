package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/subcommands"
	"github.com/levinalex/orthanctool/api"
)

const defaultInstancePageSize = 1000

type cloneCommand struct {
	source              apiFlag
	dest                apiFlag
	pollIntervalSeconds int
}

func CloneCommand() *cloneCommand { return &cloneCommand{} }

func (c *cloneCommand) Name() string { return "clone" }
func (c *cloneCommand) Usage() string {
	return `clone --orthanc <source_url> --dest <dest_url>:
	copy all instances from <source> at the orthanc installation at <dest>.` + "\n\n"
}
func (c *cloneCommand) Synopsis() string {
	return "create a complete copy of all instances in an orthanc installation"
}
func (c *cloneCommand) SetFlags(f *flag.FlagSet) {
	f.Var(&c.source, "orthanc", "source Orthanc URL")
	f.Var(&c.dest, "dest", "destination Orthanc URL")
	f.IntVar(&c.pollIntervalSeconds, "poll", 60, "poll interval in seconds")
}

func (c *cloneCommand) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if c.source.Api == nil || c.dest.Api == nil {
		return fail(fmt.Errorf("source or destination URL not set"))
	}

	err := c.run(ctx, c.source.Api, c.dest.Api)
	if err != nil {
		return fail(err)
	}

	return subcommands.ExitSuccess
}

func copyInstance(ctx context.Context, source, dest *api.Api, id string) (res api.PostInstanceResponse, err error) {
	r, len, err := source.InstanceFile(ctx, id)
	if err != nil {
		return res, err
	}
	res, err = dest.PostInstance(ctx, r, len)
	if err != nil {
		return res, err
	}
	if res.ID != id {
		return res, fmt.Errorf("instance id on destination does not match. expected %s, got %s", id, res.ID)
	}
	return res, nil
}

type StringSet struct {
	m       *sync.Mutex
	strings map[string]struct{}
}

func processExistingInstances(ctx context.Context, source *api.Api, instances chan<- string) error {
	index := 0
	for {
		fmt.Fprintf(os.Stderr, "load source instances (%d)\n", index)
		ids, err := source.Instances(ctx, index, defaultInstancePageSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			select {
			case instances <- id:
			case <-ctx.Done():
				return nil
			}
		}
		index += len(ids)
	}
	return nil
}

func loadExistingInstanceIDs(ctx context.Context, dest *api.Api, existingInstances *StringSet) error {
	index := 0
	for {
		if ctx.Err() != nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "load destination instances (%d)\n", index)
		in, err := dest.Instances(ctx, index, defaultInstancePageSize)
		if err != nil {
			return err
		}
		if len(in) == 0 {
			break
		}
		index += len(in)

		existingInstances.Add(in)
	}
	return nil
}

func processFutureChanges(ctx context.Context, source *api.Api, instances chan<- string, pollInterval time.Duration) error {
	_, lastIndex, err := source.LastChange(ctx)
	if err != nil {
		return err
	}

	err = api.ChangeWatch{
		StartIndex:   lastIndex,
		PollInterval: pollInterval,
	}.Run(ctx, source, func(cng api.ChangeResult) {
		if cng.ChangeType == "NewInstance" {
			fmt.Fprintf(os.Stderr, "%v\n", cng)
			instances <- cng.ID
		}
	})

	return err
}

func copyInstances(ctx context.Context, source, dest *api.Api, instances <-chan string, existingInstances *StringSet) error {
	for {
		select {
		case id := <-instances:
			if existingInstances.HasKey(id) {
				continue // skip existing instances
			}
			res, err := copyInstance(ctx, source, dest, id)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "copy %s %s\n", id, res.Status)
			existingInstances.Add([]string{id})
		case <-ctx.Done():
			return nil
		}
	}
}

type ErrorFunc func(err error)

func (c *cloneCommand) run(ctx context.Context, source, dest *api.Api) error {
	numUploaders := 3
	ctx, cancel := context.WithCancel(ctx)
	errors := make(chan error, 0)
	existing := NewStringSet()
	instancesToCopy := make(chan string, 0)
	wg := sync.WaitGroup{}

	wg.Add(numUploaders)
	for i := 0; i < numUploaders; i++ {
		go func() {
			defer wg.Done()
			errors <- copyInstances(ctx, source, dest, instancesToCopy, &existing)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		pollInterval := time.Duration(c.pollIntervalSeconds) * time.Second
		errors <- processFutureChanges(ctx, source, instancesToCopy, pollInterval)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errors <- loadExistingInstanceIDs(ctx, dest, &existing)
		errors <- processExistingInstances(ctx, source, instancesToCopy)
	}()

	returnError := readFirstError(errors, func() { cancel() })

	wg.Wait()
	close(errors)

	return <-returnError
}
