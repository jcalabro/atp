package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jcalabro/atmos"
	"github.com/jcalabro/atmos/identity"
	"github.com/jcalabro/atmos/plc"
	"github.com/urfave/cli/v3"
)

func plcCmd() *cli.Command {
	return &cli.Command{
		Name:  "plc",
		Usage: "PLC directory operations",
		Commands: []*cli.Command{
			plcResolveCmd(),
			plcHistoryCmd(),
		},
	}
}

func plcResolveCmd() *cli.Command {
	return &cli.Command{
		Name:      "resolve",
		Usage:     "Resolve a DID via PLC directory",
		ArgsUsage: "<did>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp plc resolve <did>")
			}

			did, err := atmos.ParseDID(c.Args().First())
			if err != nil {
				return fmt.Errorf("invalid DID: %w", err)
			}

			client := plc.NewClient(plc.ClientConfig{})
			doc, err := client.Resolve(ctx, did)
			if err != nil {
				return fmt.Errorf("resolving: %w", err)
			}

			if c.Bool("json") {
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(doc)
			}

			ident, err := identity.IdentityFromDocument(doc)
			if err != nil {
				return fmt.Errorf("parsing document: %w", err)
			}

			w := c.Root().Writer
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("DID:"), ident.DID)
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Handle:"), ident.Handle)
			if pds := ident.PDSEndpoint(); pds != "" {
				_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("PDS:"), pds)
			}
			for name, key := range ident.Keys {
				_, _ = fmt.Fprintf(w, "%s  %s (%s)\n", styleLabel.Render("Key:"), key.Multibase, name)
			}
			return nil
		},
	}
}

func plcHistoryCmd() *cli.Command {
	return &cli.Command{
		Name:      "history",
		Usage:     "Show PLC operation log for a DID",
		ArgsUsage: "<did>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp plc history <did>")
			}

			did, err := atmos.ParseDID(c.Args().First())
			if err != nil {
				return fmt.Errorf("invalid DID: %w", err)
			}

			client := plc.NewClient(plc.ClientConfig{})
			log, err := client.AuditLog(ctx, did)
			if err != nil {
				return fmt.Errorf("fetching history: %w", err)
			}

			if c.Bool("json") {
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(log)
			}

			w := c.Root().Writer
			for i, entry := range log {
				nullified := ""
				if entry.Nullified {
					nullified = styleError.Render(" (nullified)")
				}
				_, _ = fmt.Fprintf(w, "%s  %s  %s%s\n",
					styleLabel.Render(fmt.Sprintf("#%d", i+1)),
					styleDim.Render(entry.CreatedAt),
					entry.CID,
					nullified,
				)
			}
			return nil
		},
	}
}
