package main

import (
	"context"
	"flag"
	"os"

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
