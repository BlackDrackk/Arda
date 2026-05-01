package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/BlackDrackk/arda/internal/client"
	"github.com/BlackDrackk/arda/internal/config"
	"github.com/BlackDrackk/arda/internal/ui"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		conn, err := client.Connect(cfg)
		if err != nil {
			return fmt.Errorf("connecting to Podman: %w", err)
		}

		ctx, cancel := context.WithTimeout(conn, time.Duration(cfg.Timeout)*time.Second)
		defer cancel()

		opts := new(containers.ListOptions).
			WithAll(true).
			// WithSync(false) skips the OCI runtime state synchronisation,
			// reading directly from Podman's internal DB — noticeably faster
			// on machines with many containers.
			WithSync(false)

		list, err := containers.List(ctx, opts)
		if err != nil {
			return fmt.Errorf("listing containers: %w", err)
		}

		if len(list) == 0 {
			ui.Info("No containers found")
			return nil
		}

		table := ui.NewTable("NAME", "IMAGE", "STATE", "ID")
		for _, c := range list {
			name := ""
			if len(c.Names) > 0 {
				name = c.Names[0]
			}
			// Show only the first 12 chars of the ID (Docker/Podman convention).
			shortID := c.ID
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			table.AddRow(name, c.Image, ui.StateColor(c.State), shortID)
		}
		table.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
