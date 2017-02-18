package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/subcommands"
	"github.com/levinalex/orthanctool/api"
)

func main() {
	subcommands.Register(CloneCommand(), "")
	subcommands.Register(RecentPatientsCommand(), "")
	subcommands.Register(subcommands.HelpCommand(), "help")
	subcommands.Register(subcommands.FlagsCommand(), "help")
	subcommands.Register(subcommands.CommandsCommand(), "help")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}

type apiFlag struct {
	*api.Api
}

func (a *apiFlag) Set(s string) error {
	ap, err := api.New(s)
	if err == nil {
		a.Api = ap
	}
	return err
}
func (a apiFlag) String() string {
	if a.Api != nil {
		return a.BaseURL.String()
	} else {
		return ""
	}
}

func fail(e error) subcommands.ExitStatus {
	fmt.Fprintf(os.Stderr, "%s\n", e.Error())
	return subcommands.ExitFailure
}

func readFirstError(errors <-chan error, onError func()) <-chan error {
	returnError := make(chan error, 0)
	go func() {
		var firstError error
		for e := range errors {
			if e != nil && firstError == nil {
				firstError = e
				onError()
			}
		}
		returnError <- firstError
	}()
	return returnError
}

func cmdAction(cmd []string, data interface{}) error {
	b, err := json.Marshal(data)
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
