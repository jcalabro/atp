package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jcalabro/atmos"
	"github.com/jcalabro/atmos/crypto"
	"github.com/jcalabro/atmos/identity"
	"github.com/urfave/cli/v3"
)

func resolveCmd() *cli.Command {
	return &cli.Command{
		Name:      "resolve",
		Usage:     "Resolve a handle or DID to an identity",
		ArgsUsage: "<handle-or-did>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "did-only", Usage: "Print only the DID"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp resolve <handle-or-did>")
			}
			input := c.Args().First()
			jsonOut := c.Bool("json")
			didOnly := c.Bool("did-only")

			id, err := atmos.ParseATIdentifier(input)
			if err != nil {
				return fmt.Errorf("invalid identifier %q: %w", input, err)
			}

			dir := &identity.Directory{
				Resolver: &identity.DefaultResolver{},
			}

			ident, err := dir.Lookup(ctx, id)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", input, err)
			}

			if didOnly {
				_, _ = fmt.Fprintln(c.Root().Writer, ident.DID)
				return nil
			}

			if jsonOut {
				result := map[string]any{
					"did":    string(ident.DID),
					"handle": string(ident.Handle),
				}
				if pds := ident.PDSEndpoint(); pds != "" {
					result["pds"] = pds
				}
				if key, err := ident.PublicKey(); err == nil {
					result["signing_key"] = key.DIDKey()
				}
				services := map[string]any{}
				for name, svc := range ident.Services {
					services[name] = map[string]string{
						"type": svc.Type,
						"url":  svc.URL,
					}
				}
				if len(services) > 0 {
					result["services"] = services
				}
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := c.Root().Writer
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("DID:"), ident.DID)
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Handle:"), ident.Handle)
			if pds := ident.PDSEndpoint(); pds != "" {
				_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("PDS:"), pds)
			}
			if key, err := ident.PublicKey(); err == nil {
				keyType := "unknown"
				switch key.(type) {
				case *crypto.P256PublicKey:
					keyType = "P-256"
				case *crypto.K256PublicKey:
					keyType = "K-256"
				}
				_, _ = fmt.Fprintf(w, "%s  %s  (%s)\n", styleLabel.Render("Signing:"), key.DIDKey(), keyType)
			}
			return nil
		},
	}
}
