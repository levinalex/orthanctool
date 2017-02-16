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
	source apiFlag
	dest   apiFlag
}

func CloneCommand() *cloneCommand { return &cloneCommand{} }

func (c *cloneCommand) Name() string { return "clone" }
func (c *cloneCommand) Usage() string {
	return `clone --orthanc <source_url> --dest <dest_url>:
	copy all instances from <source> at the orthanc installation at <dest>.` + "\n"
}
func (c *cloneCommand) Synopsis() string {
	return "create a complete copy of all instances in an orthanc installation"
}
func (c *cloneCommand) SetFlags(f *flag.FlagSet) {
	f.Var(&c.source, "orthanc", "source Orthanc URL")
	f.Var(&c.dest, "dest", "destination Orthanc URL")
}

func (c *cloneCommand) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	fail := func(e error) subcommands.ExitStatus {
		fmt.Fprintf(os.Stderr, "%s\n", e.Error())
		return subcommands.ExitFailure
	}

	if c.source.Api == nil || c.dest.Api == nil {
		return fail(fmt.Errorf("source or destination URL not set"))
	}

	err := c.run(ctx, c.source.Api, c.dest.Api)
	if err != nil {
		return fail(err)
	}

	return subcommands.ExitSuccess
}

func copyInstance(source, dest *api.Api, id string) (res api.PostInstanceResponse, err error) {
	r, len, err := source.InstanceFile(id)
	if err != nil {
		return res, err
	}
	res, err = dest.PostInstance(r, len)
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

func processExistingInstances(ctx context.Context, wg *sync.WaitGroup, source *api.Api, instances chan<- string, e ErrorFunc) {
	defer wg.Done()

	index := 0
	for {
		fmt.Fprintf(os.Stderr, "load source instances (%d)\n", index)
		ids, err := source.Instances(index, defaultInstancePageSize)
		if err != nil {
			e(err)
			return
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			select {
			case instances <- id:
			case <-ctx.Done():
				return
			}
		}
		index += len(ids)
	}
}

func loadExistingInstanceIDs(ctx context.Context, wg *sync.WaitGroup, dest *api.Api, existingInstances *StringSet, e ErrorFunc) {
	defer wg.Done()

	index := 0
	for {
		if ctx.Err() != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "load destination instances (%d)\n", index)
		in, err := dest.Instances(index, defaultInstancePageSize)
		if err != nil {
			e(err)
			break
		}
		if len(in) == 0 {
			break
		}
		index += len(in)

		existingInstances.Add(in)
	}
}

func processFutureChanges(ctx context.Context, wg *sync.WaitGroup, source *api.Api, instances chan<- string, e ErrorFunc) {
	defer wg.Done()

	_, lastIndex, err := source.LastChange()
	if err != nil {
		e(err)
		return
	}
	cw := api.ChangeWatch{
		StartIndex:   lastIndex,
		PollInterval: 60 * time.Second,
	}
	err = cw.Run(source, ctx, func(cng api.ChangeResult) {
		if cng.ChangeType == "NewInstance" {
			fmt.Fprintf(os.Stderr, "%v\n", cng)
			instances <- cng.ID
		}
	})
	if err != nil {
		e(err)
	}
}

func instanceCopyers(threadID int, ctx context.Context, wg *sync.WaitGroup, source, dest *api.Api, instances <-chan string, existingInstances *StringSet, e ErrorFunc) {
	defer wg.Done()
	for {
		select {
		case id := <-instances:
			if existingInstances.HasKey(id) {
				continue // skip existing instances
			}
			res, err := copyInstance(source, dest, id)
			if err != nil {
				e(err)
				return
			}
			fmt.Fprintf(os.Stderr, "copy %d: %s %s\n", threadID, id, res.Status)
			existingInstances.Add([]string{id})
		case <-ctx.Done():
			return
		}
	}
}

type ErrorFunc func(err error)

func (c *cloneCommand) run(ctx context.Context, source, dest *api.Api) error {
	numUploaders := 3
	ctx, cancelFunc := context.WithCancel(ctx)
	lastError := error(nil)
	existing := NewStringSet()
	instancesToCopy := make(chan string, 0)
	wg := sync.WaitGroup{}

	e := func(err error) {
		lastError = err
		cancelFunc()
	}

	wg.Add(numUploaders)
	for i := 0; i < numUploaders; i++ {
		go instanceCopyers(i, ctx, &wg, source, dest, instancesToCopy, &existing, e)
	}

	wg.Add(1)
	go processFutureChanges(ctx, &wg, source, instancesToCopy, e)

	wg.Add(2)
	go func() {
		loadExistingInstanceIDs(ctx, &wg, dest, &existing, e)
		processExistingInstances(ctx, &wg, source, instancesToCopy, e)
	}()

	wg.Wait()
	return lastError
}
