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

var removeCmd = &cobra.Command{
	Use:   "remove <n>",
	Short: "Remove a container",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

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

		// WithForce(true): removes the container even if it is still running,
		// skipping a separate stop call entirely — significantly faster.
		opts := new(containers.RemoveOptions).WithForce(true)

		if _, err := containers.Remove(ctx, name, opts); err != nil {
			return fmt.Errorf("removing container %q: %w", name, err)
		}

		ui.Success("Container %q removed", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
