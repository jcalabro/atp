package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jcalabro/atmos/xrpc"
	"github.com/urfave/cli/v3"
)

func accountCmd() *cli.Command {
	return &cli.Command{
		Name:  "account",
		Usage: "Account login/logout/status",
		Commands: []*cli.Command{
			accountLoginCmd(),
			accountLogoutCmd(),
			accountStatusCmd(),
		},
	}
}

func accountLoginCmd() *cli.Command {
	return &cli.Command{
		Name:      "login",
		Usage:     "Login to an AT Protocol service",
		ArgsUsage: "<identifier> <password>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "host", Value: "https://bsky.social", Usage: "PDS host"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() < 2 {
				return fmt.Errorf("usage: atp account login <identifier> <password>")
			}

			identifier := c.Args().Get(0)
			password := c.Args().Get(1)
			host := c.String("host")

			client := &xrpc.Client{Host: host}
			auth, err := client.CreateSession(ctx, identifier, password)
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			sess := &sessionFile{
				Host:       host,
				AccessJwt:  auth.AccessJwt,
				RefreshJwt: auth.RefreshJwt,
				Handle:     auth.Handle,
				DID:        auth.DID,
			}

			if err := saveSession(sess); err != nil {
				return fmt.Errorf("saving session: %w", err)
			}

			_, _ = fmt.Fprintf(c.Root().Writer, "logged in as %s (%s)\n", auth.Handle, auth.DID)
			return nil
		},
	}
}

func accountLogoutCmd() *cli.Command {
	return &cli.Command{
		Name:  "logout",
		Usage: "Delete stored session",
		Action: func(ctx context.Context, c *cli.Command) error {
			client, _, err := clientFromSession()
			if err == nil {
				_ = client.DeleteSession(ctx)
			}
			if err := deleteSession(); err != nil {
				return fmt.Errorf("removing session: %w", err)
			}
			_, _ = fmt.Fprintln(c.Root().Writer, "logged out")
			return nil
		},
	}
}

func accountStatusCmd() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show current session status",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			sess, err := loadSession()
			if err != nil {
				_, _ = fmt.Fprintln(c.Root().Writer, "not logged in")
				return nil //nolint:nilerr // no session is not an error for status
			}

			if c.Bool("json") {
				result := map[string]string{
					"host":   sess.Host,
					"handle": sess.Handle,
					"did":    sess.DID,
				}
				enc := json.NewEncoder(c.Root().Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := c.Root().Writer
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Host:"), sess.Host)
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("Handle:"), sess.Handle)
			_, _ = fmt.Fprintf(w, "%s  %s\n", styleLabel.Render("DID:"), sess.DID)
			return nil
		},
	}
}
