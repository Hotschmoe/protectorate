package main

import (
	"fmt"
	"os"

	"github.com/hotschmoe/protectorate/internal/protocol"
	"github.com/urfave/cli/v2"
)

var authCommand = &cli.Command{
	Name:  "auth",
	Usage: "Manage authentication for AI providers",
	Subcommands: []*cli.Command{
		{
			Name:  "status",
			Usage: "Show authentication status for all providers",
			Action: func(c *cli.Context) error {
				return authStatusAction(c)
			},
		},
		{
			Name:      "login",
			Usage:     "Login to a provider",
			ArgsUsage: "<provider> <token>",
			Action: func(c *cli.Context) error {
				if c.NArg() < 2 {
					return cli.Exit("usage: envoy auth login <provider> <token>", 1)
				}

				provider := c.Args().Get(0)
				token := c.Args().Get(1)

				client := NewEnvoyClient(c.String("server"))
				out := NewOutputWriter(c.Bool("json"), os.Stdout)

				body := map[string]string{"token": token}

				var result protocol.AuthLoginResult
				if err := client.Post("/api/auth/"+provider+"/login", body, &result); err != nil {
					return cli.Exit(err.Error(), 1)
				}

				if c.Bool("json") {
					return out.Write(result, nil)
				}

				if result.Success {
					out.WriteMessage("%s: login successful (%s)", provider, result.Method)
				} else {
					out.WriteMessage("%s: login failed - %s", provider, result.Error)
				}
				return nil
			},
		},
		{
			Name:      "revoke",
			Usage:     "Revoke credentials for a provider",
			ArgsUsage: "<provider>",
			Action: func(c *cli.Context) error {
				if c.NArg() < 1 {
					return cli.Exit("usage: envoy auth revoke <provider>", 1)
				}

				client := NewEnvoyClient(c.String("server"))
				out := NewOutputWriter(c.Bool("json"), os.Stdout)

				provider := c.Args().First()

				var result protocol.AuthRevokeResult
				if err := client.Delete("/api/auth/" + provider); err != nil {
					return cli.Exit(err.Error(), 1)
				}

				if c.Bool("json") {
					return out.Write(result, nil)
				}

				out.WriteMessage("%s: credentials revoked", provider)
				return nil
			},
		},
	},
	Action: func(c *cli.Context) error {
		return authStatusAction(c)
	},
}

func authStatusAction(c *cli.Context) error {
	client := NewEnvoyClient(c.String("server"))
	out := NewOutputWriter(c.Bool("json"), os.Stdout)

	var status protocol.AuthStatus
	if err := client.Get("/api/auth/status", &status); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	if c.Bool("json") {
		return out.Write(status, nil)
	}

	providers := []protocol.AuthProvider{
		protocol.AuthProviderClaude,
		protocol.AuthProviderGemini,
		protocol.AuthProviderCodex,
		protocol.AuthProviderGit,
	}

	for _, provider := range providers {
		ps, ok := status.Providers[provider]
		if !ok {
			continue
		}

		if ps.Authenticated {
			method := ps.Method
			if method == "" {
				method = "unknown"
			}
			fmt.Printf("%-8s authenticated (%s)\n", provider+":", method)
		} else {
			fmt.Printf("%-8s not authenticated\n", provider+":")
		}
	}

	return nil
}
