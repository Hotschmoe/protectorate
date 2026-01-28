package main

import (
	"fmt"
	"os"

	"github.com/hotschmoe/protectorate/internal/protocol"
	"github.com/urfave/cli/v2"
)

var doctorCommand = &cli.Command{
	Name:  "doctor",
	Usage: "Run system health diagnostics",
	Action: func(c *cli.Context) error {
		client := NewEnvoyClient(c.String("server"))
		out := NewOutputWriter(c.Bool("json"), os.Stdout)

		var checks []protocol.DoctorCheck
		if err := client.Get("/api/doctor", &checks); err != nil {
			return cli.Exit(err.Error(), 1)
		}

		if c.Bool("json") {
			return out.Write(checks, nil)
		}

		hasIssues := false
		for _, check := range checks {
			status := statusIcon(check.Status)
			fmt.Fprintf(os.Stdout, "%s %s: %s\n", status, check.Name, check.Message)
			if check.Suggestion != "" && check.Status != "pass" {
				fmt.Fprintf(os.Stdout, "  -> %s\n", check.Suggestion)
				hasIssues = true
			}
		}

		if hasIssues {
			fmt.Fprintln(os.Stdout, "\nSome checks have issues. See suggestions above.")
		} else {
			fmt.Fprintln(os.Stdout, "\nAll checks passed.")
		}

		return nil
	},
}

func statusIcon(status string) string {
	switch status {
	case "pass":
		return "[OK]"
	case "warning":
		return "[!!]"
	case "fail":
		return "[XX]"
	default:
		return "[??]"
	}
}
