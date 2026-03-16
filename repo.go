package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/jcalabro/atmos"
	"github.com/jcalabro/atmos/cbor"
	"github.com/jcalabro/atmos/identity"
	"github.com/jcalabro/atmos/repo"
	"github.com/jcalabro/atmos/sync"
	"github.com/jcalabro/atmos/xrpc"
	"github.com/urfave/cli/v3"
)

func repoCmd() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "Repository operations",
		Commands: []*cli.Command{
			repoExportCmd(),
			repoInspectCmd(),
			repoLsCmd(),
		},
	}
}

func repoExportCmd() *cli.Command {
	return &cli.Command{
		Name:      "export",
		Usage:     "Download a repository as a CAR file",
		ArgsUsage: "<did-or-handle>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "Output file (default: <did>.car)"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp repo export <did-or-handle>")
			}

			input := c.Args().First()
			id, err := atmos.ParseATIdentifier(input)
			if err != nil {
				return fmt.Errorf("invalid identifier: %w", err)
			}

			dir := &identity.Directory{Resolver: &identity.DefaultResolver{}}
			ident, err := dir.Lookup(ctx, id)
			if err != nil {
				return fmt.Errorf("resolving: %w", err)
			}

			pds := ident.PDSEndpoint()
			if pds == "" {
				return fmt.Errorf("no PDS endpoint for %s", ident.DID)
			}

			sc := sync.NewClient(sync.Options{
				Client: &xrpc.Client{Host: pds},
			})

			body, err := sc.GetRepoStream(ctx, ident.DID, "")
			if err != nil {
				return fmt.Errorf("downloading repo: %w", err)
			}
			defer func() { _ = body.Close() }()

			outFile := c.String("output")
			if outFile == "" {
				outFile = string(ident.DID) + ".car"
			}

			f, err := os.Create(outFile)
			if err != nil {
				return fmt.Errorf("creating file: %w", err)
			}
			defer func() { _ = f.Close() }()

			n, err := io.Copy(f, body)
			if err != nil {
				return fmt.Errorf("writing CAR: %w", err)
			}

			_, _ = fmt.Fprintf(c.Root().Writer, "wrote %s (%d bytes)\n", outFile, n)
			return nil
		},
	}
}

func repoInspectCmd() *cli.Command {
	return &cli.Command{
		Name:      "inspect",
		Usage:     "Inspect a CAR file",
		ArgsUsage: "<car-file>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp repo inspect <car-file>")
			}

			f, err := os.Open(c.Args().First())
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			r, commit, err := repo.LoadFromCAR(bufio.NewReader(f))
			if err != nil {
				return fmt.Errorf("loading CAR: %w", err)
			}

			counts := map[string]int{}
			total := 0
			err = r.Tree.Walk(func(key string, _ cbor.CID) error {
				col := collectionFromKey(key)
				counts[col]++
				total++
				return nil
			})
			if err != nil {
				return fmt.Errorf("walking tree: %w", err)
			}

			if c.Bool("json") {
				result := map[string]any{
					"did":         commit.DID,
					"rev":         commit.Rev,
					"version":     commit.Version,
					"data":        commit.Data.String(),
					"records":     total,
					"collections": counts,
				}
				if commit.Prev != nil {
					result["prev"] = commit.Prev.String()
				}
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := c.Root().Writer
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("DID:"), commit.DID)
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Rev:"), commit.Rev)
			_, _ = fmt.Fprintf(w, "%s  %d\n", styleLabel.Render("Version:"), commit.Version)
			_, _ = fmt.Fprintf(w, "%s  %d\n", styleLabel.Render("Records:"), total)
			_, _ = fmt.Fprintln(w)

			sorted := make([]string, 0, len(counts))
			for col := range counts {
				sorted = append(sorted, col)
			}
			sort.Strings(sorted)

			for _, col := range sorted {
				_, _ = fmt.Fprintf(w, "  %-40s %d\n", col, counts[col])
			}
			return nil
		},
	}
}

func repoLsCmd() *cli.Command {
	return &cli.Command{
		Name:      "ls",
		Usage:     "List records in a CAR file",
		ArgsUsage: "<car-file> [collection]",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp repo ls <car-file> [collection]")
			}

			f, err := os.Open(c.Args().First())
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			r, _, err := repo.LoadFromCAR(bufio.NewReader(f))
			if err != nil {
				return fmt.Errorf("loading CAR: %w", err)
			}

			filter := ""
			if c.NArg() > 1 {
				filter = c.Args().Get(1)
			}
			jsonOut := c.Bool("json")

			type entry struct {
				Key string `json:"key"`
				CID string `json:"cid"`
			}

			var entries []entry
			err = r.Tree.Walk(func(key string, cid cbor.CID) error {
				if filter != "" {
					col := collectionFromKey(key)
					if col != filter {
						return nil
					}
				}
				if jsonOut {
					entries = append(entries, entry{Key: key, CID: cid.String()})
				} else {
					_, _ = fmt.Fprintf(c.Root().Writer, "%s  %s\n", key, styleDim.Render(cid.String()))
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("walking tree: %w", err)
			}

			if jsonOut {
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			return nil
		},
	}
}

func collectionFromKey(key string) string {
	for i := range key {
		if key[i] == '/' {
			return key[:i]
		}
	}
	return key
}
