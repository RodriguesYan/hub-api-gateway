package auth

import (
	"context"
	"fmt"
	"log"

	"hub-api-gateway/internal/config"

	authpb "github.com/RodriguesYan/hub-proto-contracts/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserServiceClient wraps the gRPC client for User Service
type UserServiceClient struct {
	conn   *grpc.ClientConn
	client authpb.AuthServiceClient
	config config.ServiceConfig
}

// NewUserServiceClient creates a new User Service gRPC client
func NewUserServiceClient(cfg *config.Config) (*UserServiceClient, error) {
	serviceConfig := cfg.Services["user-service"]

	log.Printf("Connecting to User Service at %s...", serviceConfig.Address)

	// Create gRPC connection (non-blocking by default with NewClient)
	conn, err := grpc.NewClient(
		serviceConfig.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user service: %w", err)
	}

	// Initiate connection (non-blocking)
	conn.Connect()

	client := authpb.NewAuthServiceClient(conn)

	log.Printf("✅ Connected to User Service at %s", serviceConfig.Address)

	return &UserServiceClient{
		conn:   conn,
		client: client,
		config: serviceConfig,
	}, nil
}

// Login calls the Login RPC method on User Service
func (c *UserServiceClient) Login(ctx context.Context, email, password string) (*authpb.LoginResponse, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	req := &authpb.LoginRequest{
		Email:    email,
		Password: password,
	}

	log.Printf("Calling User Service Login for email: %s", email)

	resp, err := c.client.Login(ctx, req)
	if err != nil {
		log.Printf("❌ Login failed: %v", err)
		return nil, fmt.Errorf("login failed: %w", err)
	}

	if !resp.ApiResponse.Success {
		log.Printf("❌ Login failed: %s", resp.ApiResponse.Message)
		return resp, fmt.Errorf("login failed: %s", resp.ApiResponse.Message)
	}

	log.Printf("✅ Login successful for email: %s", email)
	return resp, nil
}

// ValidateToken calls the ValidateToken RPC method on User Service
func (c *UserServiceClient) ValidateToken(ctx context.Context, token string) (*authpb.ValidateTokenResponse, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	req := &authpb.ValidateTokenRequest{
		Token: token,
	}

	resp, err := c.client.ValidateToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !resp.ApiResponse.Success {
		return resp, fmt.Errorf("token validation failed: %s", resp.ApiResponse.Message)
	}

	return resp, nil
}

// Close closes the gRPC connection
func (c *UserServiceClient) Close() error {
	if c.conn != nil {
		log.Println("Closing User Service gRPC connection...")
		return c.conn.Close()
	}
	return nil
}

// Ping checks if the User Service is reachable
func (c *UserServiceClient) Ping(_ context.Context) error {
	// Check the connection state
	state := c.conn.GetState()
	log.Printf("User Service connection state: %v", state)

	return nil
}
