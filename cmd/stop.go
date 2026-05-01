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

var stopTimeout uint

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a running container",
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

		// WithTimeout(0) sends SIGKILL immediately without waiting for a graceful
		// shutdown — acceptable for pentest containers that run shells or tools.
		// The --time flag lets the user override when a graceful stop is needed.
		opts := new(containers.StopOptions).WithTimeout(stopTimeout)

		if err := containers.Stop(ctx, name, opts); err != nil {
			return fmt.Errorf("stopping container %q: %w", name, err)
		}

		ui.Success("Container %q stopped", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().UintVarP(&stopTimeout, "time", "t", 0,
		"Seconds to wait before SIGKILL (0 = immediate)")
}
