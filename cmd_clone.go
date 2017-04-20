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
	"github.com/levinalex/orthanctool/stringset"
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

func existingInstances(ctx context.Context, orthanc *api.Api, instanceFunc func([]string) error) error {
	index := 0
	for {
		ids, err := orthanc.Instances(ctx, index, defaultInstancePageSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		instanceFunc(ids)
		index += len(ids)
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

func copyInstances(ctx context.Context, source, dest *api.Api, instances <-chan string, existingInstances *stringset.Set) error {
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
	pollInterval := time.Duration(c.pollIntervalSeconds) * time.Second

	ctx, cancel := context.WithCancel(ctx)
	errors := make(chan error, 0)
	returnError := readFirstError(errors, func() { cancel() })

	instancesAtDestination := stringset.New()

	instancesToCopy := make(chan string, 0)
	wg := sync.WaitGroup{}

	wg.Add(numUploaders)
	for i := 0; i < numUploaders; i++ {
		go func() {
			defer wg.Done()
			errors <- copyInstances(ctx, source, dest, instancesToCopy, &instancesAtDestination)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		errors <- processFutureChanges(ctx, source, instancesToCopy, pollInterval)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		wgSource := sync.WaitGroup{}
		instancesAtSource := stringset.New()

		wgSource.Add(1)
		go func() {
			defer wgSource.Done()

			errors <- existingInstances(ctx, source, func(ids []string) error {
				instancesAtSource.Add(ids)
				return nil
			})

		}()

		errors <- existingInstances(ctx, dest, func(ids []string) error {
			instancesAtDestination.Add(ids)
			return nil
		})

		wgSource.Wait()
		for _, id := range instancesAtSource.List() {
			select {
			case instancesToCopy <- id:
			case <-ctx.Done():
				errors <- ctx.Err()
				break
			}
		}
	}()

	wg.Wait()
	close(errors)

	return <-returnError
}
