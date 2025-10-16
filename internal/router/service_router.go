package router

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServiceRouter manages route matching and service discovery
type ServiceRouter struct {
	routes []Route
	config *RouteConfig
}

// NewServiceRouter creates a new service router from configuration file
func NewServiceRouter(configPath string) (*ServiceRouter, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes config: %w", err)
	}

	var config RouteConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse routes config: %w", err)
	}

	router := &ServiceRouter{
		config: &config,
		routes: config.Routes,
	}

	// Compile all path patterns
	for i := range router.routes {
		if err := router.routes[i].CompilePathPattern(); err != nil {
			return nil, fmt.Errorf("failed to compile route %s: %w", router.routes[i].Name, err)
		}
	}

	// Sort routes by specificity (most specific first)
	// Exact matches > Path parameters > Wildcards
	sort.Slice(router.routes, func(i, j int) bool {
		return router.calculateSpecificity(&router.routes[i]) > router.calculateSpecificity(&router.routes[j])
	})

	log.Printf("✅ Loaded %d routes from %s", len(router.routes), configPath)
	return router, nil
}

// calculateSpecificity returns a score for route specificity
// Higher score = more specific route (should be matched first)
func (r *ServiceRouter) calculateSpecificity(route *Route) int {
	score := 0

	// Exact paths (no variables or wildcards) get highest priority
	if !strings.Contains(route.Path, "{") && !strings.Contains(route.Path, "*") {
		score += 1000
	}

	// Path parameters are next
	if strings.Contains(route.Path, "{") {
		score += 500
	}

	// Wildcards are lowest priority
	if strings.Contains(route.Path, "*") {
		score += 100
	}

	// Longer paths are more specific
	score += len(route.Path)

	// Routes with specific methods are more specific
	if route.Method != "" {
		score += 50
	}

	return score
}

// FindRoute finds a matching route for the given path and method
func (r *ServiceRouter) FindRoute(path, method string) (*Route, error) {
	for i := range r.routes {
		route := &r.routes[i]
		if route.Matches(path, method) {
			log.Printf("📍 Route matched: %s %s -> %s", method, path, route.Name)
			return route, nil
		}
	}

	return nil, fmt.Errorf("no route found for %s %s", method, path)
}

// GetRoutes returns all configured routes
func (r *ServiceRouter) GetRoutes() []Route {
	return r.routes
}

// GetRoutesByService returns all routes for a specific service
func (r *ServiceRouter) GetRoutesByService(serviceName string) []Route {
	var routes []Route
	for _, route := range r.routes {
		if route.Service == serviceName {
			routes = append(routes, route)
		}
	}
	return routes
}

// GetProtectedRoutes returns all routes that require authentication
func (r *ServiceRouter) GetProtectedRoutes() []Route {
	var routes []Route
	for _, route := range r.routes {
		if route.AuthRequired {
			routes = append(routes, route)
		}
	}
	return routes
}

// GetPublicRoutes returns all routes that don't require authentication
func (r *ServiceRouter) GetPublicRoutes() []Route {
	var routes []Route
	for _, route := range r.routes {
		if !route.AuthRequired {
			routes = append(routes, route)
		}
	}
	return routes
}

// ListRoutes logs all configured routes for debugging
func (r *ServiceRouter) ListRoutes() {
	log.Println("📋 Configured Routes:")
	log.Println("=====================================================")

	serviceRoutes := make(map[string][]Route)
	for _, route := range r.routes {
		serviceRoutes[route.Service] = append(serviceRoutes[route.Service], route)
	}

	for serviceName, routes := range serviceRoutes {
		log.Printf("\n🔹 %s:", serviceName)
		for _, route := range routes {
			auth := "🔓 public"
			if route.AuthRequired {
				auth = "🔒 protected"
			}
			log.Printf("  %s %s -> %s.%s (%s)",
				route.Method, route.Path, route.GRPCService, route.GRPCMethod, auth)
		}
	}

	log.Println("\n=====================================================")
	log.Printf("Total: %d routes (%d protected, %d public)\n",
		len(r.routes),
		len(r.GetProtectedRoutes()),
		len(r.GetPublicRoutes()))
}
