package cmd

import (
	"context"
	"fmt"
	"os"
	"time"
	"github.com/BlackDrackk/arda/internal/client"
	"github.com/BlackDrackk/arda/internal/config"
	"github.com/BlackDrackk/arda/internal/ui"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/spf13/cobra"
)

var networkFlag string

var startCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start or re-enter an Arda container",
	Long: `Start a new Arda container or re-enter an existing one.

If the container does not exist it is created from the configured image.
If it exists but is stopped it is restarted. In both cases a shell is
attached immediately after.

Network mode can be overridden per invocation with --network:
  arda start pentest --network host
  arda start isolated --network none
  arda start audit   --network slirp4netns`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// ── Config ────────────────────────────────────────────────────────────
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// --network overrides config.toml only when the flag is explicitly set.
		if cmd.Flags().Changed("network") {
			cfg.Network = config.NetworkMode(networkFlag)
		}

		// ── Podman connection ─────────────────────────────────────────────────
		// conn holds the socket handle for all API calls.
		// A short-lived timeout context is used for setup operations only.
		// Attach receives conn directly so the interactive session never times out.
		conn, err := client.Connect(cfg)
		if err != nil {
			return fmt.Errorf("connecting to Podman: %w", err)
		}

		ctx, cancel := context.WithTimeout(conn, time.Duration(cfg.Timeout)*time.Second)
		defer cancel()

		// ── Container lifecycle ───────────────────────────────────────────────
		exists, err := containers.Exists(ctx, name, nil)
		if err != nil {
			return fmt.Errorf("checking container existence: %w", err)
		}

		if !exists {
			// ── First run: create and start ───────────────────────────────────
			ui.Info("Creating container %q (image: %s, network: %s)",
				name, cfg.DefaultImage, cfg.Network)

			report, err := containers.CreateWithSpec(ctx, newSecureSpec(name, cfg), nil)
			if err != nil {
				return fmt.Errorf("creating container: %w", err)
			}

			if err := containers.Start(ctx, report.ID, nil); err != nil {
				return fmt.Errorf("starting container: %w", err)
			}

			ui.Success("Container %q ready", name)
		} else {
			// ── Subsequent runs: restart if needed then re-attach ─────────────
			inspect, err := containers.Inspect(ctx, name, nil)
			if err != nil {
				return fmt.Errorf("inspecting container: %w", err)
			}

			if inspect.State.Status != "running" {
				ui.Info("Restarting container %q", name)
				if err := containers.Start(ctx, name, nil); err != nil {
					return fmt.Errorf("restarting container: %w", err)
				}
			}

			ui.Success("Re-entering container %q", name)
		}

		// ── Attach ────────────────────────────────────────────────────────────
		// conn has no timeout — the shell session can last as long as needed.
		// When the user types exit, bash terminates, the container moves to
		// "exited", and the next `arda start <name>` will restart it cleanly.
		if err := containers.Attach(conn, name, os.Stdin, os.Stdout, os.Stderr, nil, nil); err != nil {
			return fmt.Errorf("attaching to container: %w", err)
		}

		return nil
	},
}

// newSecureSpec returns a hardened SpecGenerator for a new Arda container.
//
// Security model:
//   - Rootless Podman is the primary isolation boundary: the container's root
//     maps to your unprivileged WSL2 user on the host — no real host privileges.
//   - NoNewPrivileges blocks privilege escalation via setuid/setgid binaries.
//   - /bin/bash is the sole entrypoint; the container exits when the shell exits,
//     keeping the lifecycle simple and predictable.
func newSecureSpec(name string, cfg config.Config) *specgen.SpecGenerator {
	s := specgen.NewSpecGenerator(cfg.DefaultImage, false)

	s.Name = name
	s.Terminal = true
	s.NetNS = networkNamespace(cfg.Network)
	s.NoNewPrivileges = false

	// /bin/bash as entrypoint: Attach connects directly to the shell.
	// When the user exits, the container stops — arda start restarts it next time.
	s.Entrypoint = []string{"/bin/bash"}

	return s
}

// networkNamespace maps a NetworkMode to the specgen.Namespace expected by the
// Podman API.
func networkNamespace(mode config.NetworkMode) specgen.Namespace {
	switch mode {
	case config.NetworkHost:
		// Shares the host network stack — full access, no isolation.
		return specgen.Namespace{NSMode: specgen.Host}
	case config.NetworkNone:
		// Loopback only — maximum network isolation.
		return specgen.Namespace{NSMode: specgen.NoNetwork}
	case config.NetworkSlirp:
		// Userspace TCP/IP — rootless-friendly, no kernel privileges needed.
		return specgen.Namespace{NSMode: specgen.Slirp}
	default:
		// Bridge — isolated virtual network with NAT (recommended default).
		return specgen.Namespace{NSMode: specgen.Bridge}
	}
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVarP(
		&networkFlag, "network", "n", "",
		"Network mode: bridge | host | none | slirp4netns (default: from config)",
	)
}