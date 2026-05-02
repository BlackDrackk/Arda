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

		table := ui.NewTable("NAME", "IMAGE", "STATE", "NETWORK", "TYPE", "ID")
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
			
			network := string(config.NetworkBridge)
			contype := "podman"

			if len(c.Networks) > 0 {
				switch c.Networks[0] {
				case string(config.NetworkHost):
					network = string(config.NetworkHost)
				case string(config.NetworkNone), "":
					network = string(config.NetworkNone)
				case string(config.NetworkSlirp):
					network = string(config.NetworkSlirp)
				default:
					network = string(config.NetworkBridge)
				}
				// docker containers ont un réseau préfixé par le nom du projet
				if c.Labels["io.podman.compose.project"] == "" && c.Labels["com.docker.compose.project"] != "" {
					contype = "docker"
				}
			}
			table.AddRow(name, c.Image, ui.StateColor(c.State), network, contype, shortID)
		}
		table.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
