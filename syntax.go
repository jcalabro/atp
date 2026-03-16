package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jcalabro/atmos"
	"github.com/jcalabro/atmos/cbor"
	"github.com/urfave/cli/v3"
)

func syntaxCmd() *cli.Command {
	return &cli.Command{
		Name:      "syntax",
		Usage:     "Validate AT Protocol syntax types",
		ArgsUsage: "<type> <value>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 2 {
				return fmt.Errorf("usage: atp syntax <type> <value>")
			}
			typ := c.Args().Get(0)
			val := c.Args().Get(1)
			jsonOut := c.Bool("json")

			var err error
			var parsed string

			switch typ {
			case "did":
				d, e := atmos.ParseDID(val)
				err, parsed = e, string(d)
			case "handle":
				h, e := atmos.ParseHandle(val)
				err, parsed = e, string(h)
			case "nsid":
				n, e := atmos.ParseNSID(val)
				err, parsed = e, string(n)
			case "at-uri", "aturi":
				a, e := atmos.ParseATURI(val)
				err, parsed = e, string(a)
			case "tid":
				t, e := atmos.ParseTID(val)
				err, parsed = e, string(t)
			case "record-key", "rkey":
				r, e := atmos.ParseRecordKey(val)
				err, parsed = e, string(r)
			case "datetime":
				d, e := atmos.ParseDatetime(val)
				err, parsed = e, string(d)
			case "language":
				l, e := atmos.ParseLanguage(val)
				err, parsed = e, string(l)
			case "uri":
				u, e := atmos.ParseURI(val)
				err, parsed = e, string(u)
			case "cid":
				ci, e := cbor.ParseCIDString(val)
				err, parsed = e, ci.String()
			default:
				return fmt.Errorf("unknown syntax type %q (valid: did, handle, nsid, at-uri, tid, record-key, datetime, language, uri, cid)", typ)
			}

			if jsonOut {
				result := map[string]any{
					"type":  typ,
					"input": val,
					"valid": err == nil,
				}
				if err != nil {
					result["error"] = err.Error()
				} else {
					result["normalized"] = parsed
				}
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			if err != nil {
				_, _ = fmt.Fprintf(c.Root().Writer, "%s  invalid %s: %v\n", styleError.Render("✗"), typ, err)
				return err
			}
			_, _ = fmt.Fprintf(c.Root().Writer, "%s  valid %s", styleCreate.Render("✓"), typ)
			if parsed != val {
				_, _ = fmt.Fprintf(c.Root().Writer, " (normalized: %s)", parsed)
			}
			_, _ = fmt.Fprintln(c.Root().Writer)
			return nil
		},
	}
}
