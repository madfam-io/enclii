package cloudflare

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// GetTunnel retrieves tunnel information by ID
func (c *Client) GetTunnel(ctx context.Context, tunnelID string) (*Tunnel, error) {
	if tunnelID == "" {
		tunnelID = c.tunnelID
	}

	if tunnelID == "" {
		return nil, fmt.Errorf("cloudflare: tunnel ID is required")
	}

	var resp APIResponse[Tunnel]
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", c.accountID, tunnelID)

	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("failed to get tunnel %s: %w", tunnelID, err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s", resp.Errors[0].Message)
		}
		return nil, fmt.Errorf("unknown API error")
	}

	return &resp.Result, nil
}

// GetTunnelConnections retrieves active connections for a tunnel
func (c *Client) GetTunnelConnections(ctx context.Context, tunnelID string) ([]TunnelConnection, error) {
	if tunnelID == "" {
		tunnelID = c.tunnelID
	}

	if tunnelID == "" {
		return nil, fmt.Errorf("cloudflare: tunnel ID is required")
	}

	var resp APIResponse[[]TunnelConnection]
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/connections", c.accountID, tunnelID)

	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("failed to get tunnel connections: %w", err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s", resp.Errors[0].Message)
		}
		return nil, fmt.Errorf("unknown API error")
	}

	return resp.Result, nil
}

// GetTunnelStatus returns an aggregated status for the tunnel
func (c *Client) GetTunnelStatus(ctx context.Context, tunnelID string) (*TunnelStatus, error) {
	if tunnelID == "" {
		tunnelID = c.tunnelID
	}

	// Get tunnel details
	tunnel, err := c.GetTunnel(ctx, tunnelID)
	if err != nil {
		return nil, err
	}

	// Get connections
	connections, err := c.GetTunnelConnections(ctx, tunnelID)
	if err != nil {
		logrus.WithError(err).Warn("Failed to get tunnel connections, using tunnel status only")
		connections = []TunnelConnection{}
	}

	// Count active connections and collect colos
	activeCount := 0
	colos := make([]string, 0)
	var lastHealthy time.Time

	for _, conn := range connections {
		if !conn.IsDeleted {
			activeCount++
			colos = append(colos, conn.ColoName)
			if conn.OpenedAt.After(lastHealthy) {
				lastHealthy = conn.OpenedAt
			}
		}
	}

	// Determine status
	status := "inactive"
	if tunnel.Status == "healthy" {
		status = "active"
	} else if tunnel.Status == "degraded" || (activeCount > 0 && activeCount < len(connections)) {
		status = "degraded"
	} else if activeCount > 0 {
		status = "active"
	}

	// Use tunnel's last active time if no connections
	if lastHealthy.IsZero() && tunnel.ConnsActiveAt != nil {
		lastHealthy = *tunnel.ConnsActiveAt
	}

	result := &TunnelStatus{
		TunnelID:         tunnel.ID,
		TunnelName:       tunnel.Name,
		Status:           status,
		ActiveConnectors: activeCount,
		TotalConnectors:  len(connections),
		LastHealthy:      lastHealthy,
		Colos:            colos,
	}

	logrus.WithFields(logrus.Fields{
		"tunnel_id":   tunnel.ID,
		"tunnel_name": tunnel.Name,
		"status":      status,
		"connectors":  activeCount,
	}).Debug("Retrieved tunnel status from Cloudflare")

	return result, nil
}

// ListTunnels retrieves all tunnels for the account
func (c *Client) ListTunnels(ctx context.Context) ([]Tunnel, error) {
	var resp APIResponse[[]Tunnel]
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel", c.accountID)

	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("failed to list tunnels: %w", err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s", resp.Errors[0].Message)
		}
		return nil, fmt.Errorf("unknown API error")
	}

	return resp.Result, nil
}
