package proxy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"hub-api-gateway/internal/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ServiceRegistry manages gRPC connections to microservices
type ServiceRegistry struct {
	connections map[string]*grpc.ClientConn
	config      *config.Config
	mu          sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(cfg *config.Config) *ServiceRegistry {
	return &ServiceRegistry{
		connections: make(map[string]*grpc.ClientConn),
		config:      cfg,
	}
}

// GetConnection returns a gRPC connection for the given service name
// Creates a new connection if one doesn't exist (lazy loading)
func (r *ServiceRegistry) GetConnection(serviceName string) (*grpc.ClientConn, error) {
	r.mu.RLock()
	conn, exists := r.connections[serviceName]
	r.mu.RUnlock()

	if exists && conn.GetState().String() != "SHUTDOWN" {
		return conn, nil
	}

	return r.createConnection(serviceName)
}

// createConnection creates a new gRPC connection to a service
func (r *ServiceRegistry) createConnection(serviceName string) (*grpc.ClientConn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check if connection was created while waiting for lock
	if conn, exists := r.connections[serviceName]; exists && conn.GetState().String() != "SHUTDOWN" {
		return conn, nil
	}

	serviceConfig, exists := r.config.Services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found in configuration", serviceName)
	}

	log.Printf("🔌 Creating gRPC connection to %s at %s", serviceName, serviceConfig.Address)

	// gRPC dial options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
			grpc.MaxCallSendMsgSize(10*1024*1024), // 10MB
		),
	}

	conn, err := grpc.NewClient(serviceConfig.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client for %s: %w", serviceName, err)
	}

	conn.Connect()

	r.connections[serviceName] = conn
	log.Printf("✅ Connected to %s", serviceName)

	return conn, nil
}

// Close closes all gRPC connections
func (r *ServiceRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Println("🔌 Closing all service connections...")

	var errors []error
	for serviceName, conn := range r.connections {
		if err := conn.Close(); err != nil {
			log.Printf("⚠️  Error closing connection to %s: %v", serviceName, err)
			errors = append(errors, err)
		} else {
			log.Printf("✅ Closed connection to %s", serviceName)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to close %d connections", len(errors))
	}

	return nil
}

// HealthCheck checks if a service connection is healthy
func (r *ServiceRegistry) HealthCheck(serviceName string) error {
	conn, err := r.GetConnection(serviceName)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	state := conn.GetState()
	if state.String() == "SHUTDOWN" || state.String() == "TRANSIENT_FAILURE" {
		return fmt.Errorf("connection to %s is not healthy: %s", serviceName, state)
	}

	// Wait for connection to be ready
	if !conn.WaitForStateChange(ctx, state) {
		return fmt.Errorf("connection to %s did not become ready", serviceName)
	}

	return nil
}

// GetAllServices returns a list of all registered service names
func (r *ServiceRegistry) GetAllServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.connections))
	for serviceName := range r.connections {
		services = append(services, serviceName)
	}

	return services
}

// GetConnectionState returns the connection state for a service
func (r *ServiceRegistry) GetConnectionState(serviceName string) (string, error) {
	r.mu.RLock()
	conn, exists := r.connections[serviceName]
	r.mu.RUnlock()

	if !exists {
		return "NOT_CONNECTED", nil
	}

	return conn.GetState().String(), nil
}
