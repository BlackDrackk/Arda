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

// networkFlag holds the value of the --network flag.
// Empty string means "use the value from config.toml".
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
  arda start isolated --network none`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// ── Config ────────────────────────────────────────────────────────────
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// --network flag overrides config.toml when explicitly provided.
		if cmd.Flags().Changed("network") {
			cfg.Network = config.NetworkMode(networkFlag)
		}

		// ── Podman connection ─────────────────────────────────────────────────
		// conn carries the socket handle used for all API calls.
		// A timeout context is derived for setup operations (create, inspect…);
		// Attach reuses conn directly so a long session never times out.
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
			// ── Create + start ────────────────────────────────────────────────
			ui.Info("Creating container %q (image: %s, network: %s)",
				name, cfg.DefaultImage, cfg.Network)

			s := newSecureSpec(name, cfg)

			report, err := containers.CreateWithSpec(ctx, s, nil)
			if err != nil {
				return fmt.Errorf("creating container: %w", err)
			}

			if err := containers.Start(ctx, report.ID, nil); err != nil {
				return fmt.Errorf("starting container: %w", err)
			}

			ui.Success("Container %q ready", name)
		} else {
			// ── Re-enter existing container ───────────────────────────────────
			inspect, err := containers.Inspect(ctx, name, nil)
			if err != nil {
				return fmt.Errorf("inspecting container: %w", err)
			}

			if inspect.State.Status != "running" {
				ui.Info("Restarting container %q", name)
				if err := containers.Start(ctx, name, nil); err != nil {
					return fmt.Errorf("starting existing container: %w", err)
				}
			}

			ui.Success("Re-entering container %q", name)
		}

		// ── Attach ────────────────────────────────────────────────────────────
		// conn is used directly (no timeout) so the session can last indefinitely.
		if err := containers.Attach(conn, name, os.Stdin, os.Stdout, os.Stderr, nil, nil); err != nil {
			return fmt.Errorf("attaching to container: %w", err)
		}

		return nil
	},
}

// newSecureSpec builds a hardened SpecGenerator for a new Arda container.
//
// Security choices:
//   - All Linux capabilities are dropped; only NET_RAW and NET_ADMIN are added
//     back because most network audit tools (ping, tcpdump, nmap raw sockets…)
//     require them. Remove these if your workload does not need raw sockets.
//   - NoNewPrivileges prevents setuid/setgid binaries inside the container from
//     gaining extra privileges (e.g. sudo, su).
//   - The container runs as root inside its own user namespace (rootless Podman),
//     which means it has no real root privileges on the host.
func newSecureSpec(name string, cfg config.Config) *specgen.SpecGenerator {
	s := specgen.NewSpecGenerator(cfg.DefaultImage, false)

	s.Name = name
	s.Terminal = true
	s.NetNS = networkNamespace(cfg.Network)

	// Drop every capability then add back the minimum required.
	s.CapDrop = []string{"ALL"}
	s.CapAdd = []string{
		"NET_RAW",   // raw sockets — required by ping, tcpdump, nmap SYN scans
		"NET_ADMIN", // interface config — required by ip, iptables, traffic control
	}

	// Prevent privilege escalation via setuid / setgid binaries.
	s.NoNewPrivileges = true

	return s
}

// networkNamespace maps a config.NetworkMode to the specgen.Namespace value
// understood by the Podman API.
func networkNamespace(mode config.NetworkMode) specgen.Namespace {
	switch mode {
	case config.NetworkHost:
		// Shares the host network stack — full access, no isolation.
		return specgen.Namespace{NSMode: specgen.Host}
	case config.NetworkNone:
		// No network interface at all — maximum isolation.
		return specgen.Namespace{NSMode: specgen.NoNetwork}
	case config.NetworkSlirp:
		// Userspace TCP/IP stack — rootless-friendly, no kernel privileges.
		return specgen.Namespace{NSMode: specgen.Slirp}
	default:
		// Bridge (default) — isolated virtual network with NAT.
		return specgen.Namespace{NSMode: specgen.Bridge}
	}
}

func init() {
	rootCmd.AddCommand(startCmd)

	// --network / -n: override the network mode from config.toml.
	startCmd.Flags().StringVarP(
		&networkFlag, "network", "n", "",
		`Network mode: bridge | host | none | slirp4netns (default: from config)`,
	)
}