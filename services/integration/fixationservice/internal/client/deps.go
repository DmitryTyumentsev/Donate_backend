package client

import (
	"Broker_backend/services/integration/fixationservice/internal/config"
	"context"
	"errors"
	"fmt"

	authv1 "Broker_backend/gen/auth/v1"
	grpcauth "Broker_backend/shared/pkg/grpc/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	auth   authv1.AuthServiceClient
	config *config.Config
}

func NewFixationServiceClient(cfg *config.Config) (*grpc.ClientConn, authv1.AuthServiceClient, error) {
	if cfg == nil {
		return nil, nil, errors.New("config is nil")
	}

	if cfg.AuthGRPC.Address == "" {
		return nil, nil, errors.New("auth_grpc.address is required")
	}

	conn, err := grpc.NewClient(
		"passthrough:///"+cfg.AuthGRPC.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create auth service client: %w", err)
	}

	return conn, authv1.NewAuthServiceClient(conn), nil
}

func NewClient(auth authv1.AuthServiceClient, cfg *config.Config) (*Client, error) {
	client := &Client{
		auth:   auth,
		config: cfg,
	}

	if err := client.Validate(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Validate() error {
	switch {
	case c == nil:
		return errors.New("client is nil")
	case c.auth == nil:
		return errors.New("auth grpc client is nil")
	case c.config == nil:
		return errors.New("config is nil")
	case c.config.OperationTimeout() <= 0:
		return errors.New("operation timeout must be positive")
	default:
		return nil
	}
}

func (c *Client) contextWithTimeout(parent context.Context) (context.Context, context.CancelFunc, error) {
	if err := c.Validate(); err != nil {
		return nil, nil, err
	}

	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithTimeout(parent, c.config.OperationTimeout())
	ctx = grpcauth.InjectOutgoingContext(ctx)

	return ctx, cancel, nil
}
