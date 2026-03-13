package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jcalabro/atmos/lexicon"
	"github.com/jcalabro/atmos/lexval"
	"github.com/urfave/cli/v3"
)

func validateCmd() *cli.Command {
	return &cli.Command{
		Name:      "validate",
		Usage:     "Validate a JSON record against a lexicon schema",
		ArgsUsage: "<collection> <json-file>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "lexdir", Usage: "Directory containing lexicon JSON files", Value: ""},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 2 {
				return fmt.Errorf("usage: atp validate <collection> <json-file>")
			}

			collection := c.Args().Get(0)
			jsonFile := c.Args().Get(1)
			lexDir := c.String("lexdir")
			jsonOut := c.Bool("json")

			data, err := os.ReadFile(jsonFile)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			var record map[string]any
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parsing JSON: %w", err)
			}

			if lexDir == "" {
				// Try default lexicon directory relative to binary
				candidates := []string{
					"lexicons",
					"../atmos/lexicons",
				}
				for _, dir := range candidates {
					if info, err := os.Stat(dir); err == nil && info.IsDir() {
						lexDir = dir
						break
					}
				}
				if lexDir == "" {
					return fmt.Errorf("no lexicon directory found; use --lexdir")
				}
			}

			schemas, err := lexicon.ParseDir(lexDir)
			if err != nil {
				return fmt.Errorf("parsing lexicons: %w", err)
			}

			cat := lexicon.NewCatalog()
			for _, s := range schemas {
				if err := cat.Add(s); err != nil {
					return fmt.Errorf("adding schema %s: %w", s.ID, err)
				}
			}
			if err := cat.Resolve(); err != nil {
				return fmt.Errorf("resolving schemas: %w", err)
			}

			err = lexval.ValidateRecord(cat, collection, record)

			if jsonOut {
				result := map[string]any{
					"collection": collection,
					"valid":      err == nil,
				}
				if err != nil {
					result["error"] = err.Error()
				}
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := c.Root().Writer
			if err == nil {
				_, _ = fmt.Fprintf(w, "%s  valid %s record\n", styleCreate.Render("✓"), collection)
				return nil
			}

			_, _ = fmt.Fprintf(w, "%s  %v\n", styleError.Render("✗"), err)
			return err
		},
	}
}
