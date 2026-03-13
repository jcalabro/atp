package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jcalabro/atmos"
	"github.com/jcalabro/atmos/api/comatproto"
	"github.com/jcalabro/atmos/identity"
	"github.com/jcalabro/atmos/xrpc"
	"github.com/urfave/cli/v3"
)

func recordCmd() *cli.Command {
	return &cli.Command{
		Name:  "record",
		Usage: "Fetch and list records",
		Commands: []*cli.Command{
			recordGetCmd(),
			recordListCmd(),
		},
	}
}

func recordGetCmd() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Fetch a record by AT-URI",
		ArgsUsage: "<at-uri>",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp record get <at-uri>")
			}

			uri, err := atmos.ParseATURI(c.Args().First())
			if err != nil {
				return fmt.Errorf("invalid AT-URI: %w", err)
			}

			pds, err := resolvePDS(ctx, string(uri.Authority()))
			if err != nil {
				return err
			}

			client := &xrpc.Client{Host: pds}
			out, err := comatproto.RepoGetRecord(ctx, client, "", string(uri.Collection()), string(uri.Authority()), string(uri.RecordKey()))
			if err != nil {
				return fmt.Errorf("fetching record: %w", err)
			}

			enc := json.NewEncoder(c.Root().Writer)
			enc.SetIndent("", "  ")
			return enc.Encode(out.Value)
		},
	}
}

func recordListCmd() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List records for a repo, optionally filtered by collection",
		ArgsUsage: "<did-or-handle> [collection]",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "limit", Value: 50, Usage: "Max records to return"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp record list <did-or-handle> [collection]")
			}
			repo := c.Args().Get(0)
			collection := c.Args().Get(1)
			limit := int64(c.Int("limit"))
			jsonOut := c.Bool("json")

			if collection == "" {
				return listCollections(ctx, c, repo, jsonOut)
			}

			pds, err := resolvePDS(ctx, repo)
			if err != nil {
				return err
			}

			client := &xrpc.Client{Host: pds}
			out, err := comatproto.RepoListRecords(ctx, client, collection, "", limit, repo, false)
			if err != nil {
				return fmt.Errorf("listing records: %w", err)
			}

			if jsonOut {
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(out.Records)
			}

			w := c.Root().Writer
			for _, rec := range out.Records {
				_, _ = fmt.Fprintf(w, "%s  %s\n", rec.URI, styleDim.Render(rec.CID))
			}
			if len(out.Records) == 0 {
				_, _ = fmt.Fprintln(w, "no records found")
			}
			return nil
		},
	}
}

func listCollections(ctx context.Context, c *cli.Command, repo string, jsonOut bool) error {
	pds, err := resolvePDS(ctx, repo)
	if err != nil {
		return err
	}

	client := &xrpc.Client{Host: pds}
	out, err := comatproto.RepoDescribeRepo(ctx, client, repo)
	if err != nil {
		return fmt.Errorf("describing repo: %w", err)
	}

	if jsonOut {
		enc := json.NewEncoder(c.Root().Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(out.Collections)
	}

	w := c.Root().Writer
	for _, col := range out.Collections {
		_, _ = fmt.Fprintln(w, col)
	}
	return nil
}

func resolvePDS(ctx context.Context, input string) (string, error) {
	id, err := atmos.ParseAtIdentifier(input)
	if err != nil {
		return "", fmt.Errorf("invalid identifier %q: %w", input, err)
	}

	dir := &identity.Directory{
		Resolver: &identity.DefaultResolver{},
	}

	ident, err := dir.Lookup(ctx, id)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", input, err)
	}

	pds := ident.PDSEndpoint()
	if pds == "" {
		return "", fmt.Errorf("no PDS endpoint found for %q", input)
	}
	return pds, nil
}
