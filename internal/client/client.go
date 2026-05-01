// Package client manages the connection to the Podman REST API.
package client

import (
	"context"
	"fmt"

	"github.com/BlackDrackk/arda/internal/config"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/system"
)

// Connect opens a new connection to the Podman socket defined in cfg
// and returns a context carrying the connection handle.
func Connect(cfg config.Config) (context.Context, error) {
	conn, err := bindings.NewConnection(context.Background(), cfg.Socket)
	if err != nil {
		return nil, fmt.Errorf("podman connect (%s): %w", cfg.Socket, err)
	}
	return conn, nil
}

// Ping verifies that the Podman daemon is reachable by fetching system info.
// Using system.Info is semantically correct; containers.List was previously
// used as a workaround.
func Ping(ctx context.Context) error {
	if _, err := system.Info(ctx, nil); err != nil {
		return fmt.Errorf("podman ping: %w", err)
	}
	return nil
}
