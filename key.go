package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jcalabro/atmos/crypto"
	"github.com/urfave/cli/v3"
)

func keyCmd() *cli.Command {
	return &cli.Command{
		Name:  "key",
		Usage: "Key generation and inspection",
		Commands: []*cli.Command{
			keyGenerateCmd(),
			keyInspectCmd(),
		},
	}
}

func keyGenerateCmd() *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Generate a new signing key pair",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Value: "p256", Usage: "Key type: p256 or k256"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			keyType := c.String("type")
			jsonOut := c.Bool("json")

			var priv crypto.PrivateKey
			var err error

			switch keyType {
			case "p256", "P-256", "P256":
				priv, err = crypto.GenerateP256()
				keyType = "P-256"
			case "k256", "K-256", "K256", "secp256k1":
				priv, err = crypto.GenerateK256()
				keyType = "K-256"
			default:
				return fmt.Errorf("unknown key type %q (valid: p256, k256)", keyType)
			}
			if err != nil {
				return fmt.Errorf("generating key: %w", err)
			}

			pub := priv.PublicKey()

			if jsonOut {
				result := map[string]any{
					"type":      keyType,
					"did_key":   pub.DIDKey(),
					"multibase": pub.Multibase(),
					"public":    hex.EncodeToString(pub.Bytes()),
				}
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := c.Root().Writer
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Type:"), keyType)
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("DID Key:"), pub.DIDKey())
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Multibase:"), pub.Multibase())
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Public:"), hex.EncodeToString(pub.Bytes()))
			return nil
		},
	}
}

func keyInspectCmd() *cli.Command {
	return &cli.Command{
		Name:      "inspect",
		Usage:     "Inspect a did:key or multibase public key",
		ArgsUsage: "<did-key-or-multibase>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 1 {
				return fmt.Errorf("usage: atp key inspect <did-key-or-multibase>")
			}
			input := c.Args().First()
			jsonOut := c.Bool("json")

			var pub crypto.PublicKey
			var err error

			if len(input) > 8 && input[:8] == "did:key:" {
				pub, err = crypto.ParsePublicDIDKey(input)
			} else {
				pub, err = crypto.ParsePublicMultibase(input)
			}
			if err != nil {
				return fmt.Errorf("parsing key: %w", err)
			}

			keyType := "unknown"
			switch pub.(type) {
			case *crypto.P256PublicKey:
				keyType = "P-256"
			case *crypto.K256PublicKey:
				keyType = "K-256"
			}

			if jsonOut {
				result := map[string]any{
					"type":      keyType,
					"did_key":   pub.DIDKey(),
					"multibase": pub.Multibase(),
					"public":    hex.EncodeToString(pub.Bytes()),
				}
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := c.Root().Writer
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Type:"), keyType)
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("DID Key:"), pub.DIDKey())
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Multibase:"), pub.Multibase())
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Public:"), hex.EncodeToString(pub.Bytes()))
			return nil
		},
	}
}
