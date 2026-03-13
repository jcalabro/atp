package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "atp",
		Usage: "AT Protocol command-line tool",
		Commands: []*cli.Command{
			subscribeCmd(),
			resolveCmd(),
			recordCmd(),
			repoCmd(),
			syntaxCmd(),
			keyCmd(),
			plcCmd(),
			validateCmd(),
			accountCmd(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
