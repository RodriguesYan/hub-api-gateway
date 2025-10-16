package router

import (
	"fmt"
	"regexp"
	"strings"
)

// Route represents a single routing rule
type Route struct {
	Name         string           `yaml:"name"`
	Path         string           `yaml:"path"`
	Method       string           `yaml:"method"`
	Service      string           `yaml:"service"`
	GRPCService  string           `yaml:"grpc_service"`
	GRPCMethod   string           `yaml:"grpc_method"`
	AuthRequired bool             `yaml:"auth_required"`
	RateLimit    *RateLimitConfig `yaml:"rate_limit,omitempty"`
	Timeout      string           `yaml:"timeout,omitempty"`
	Description  string           `yaml:"description,omitempty"`

	// Compiled regex for path matching (used internally)
	pathRegex *regexp.Regexp
	pathVars  []string // Variable names extracted from path (e.g., ["id", "symbol"])
}

// RateLimitConfig defines rate limiting parameters
type RateLimitConfig struct {
	Requests int    `yaml:"requests"`
	Per      string `yaml:"per"` // "second", "minute", "hour"
}

// RouteConfig holds all routes
type RouteConfig struct {
	Routes []Route `yaml:"routes"`
}

// CompilePathPattern compiles the path pattern into a regex for matching
func (r *Route) CompilePathPattern() error {
	pattern := r.Path

	// Extract path variables (e.g., /orders/{id} -> ["id"])
	varPattern := regexp.MustCompile(`\{([^}]+)\}`)
	matches := varPattern.FindAllStringSubmatch(pattern, -1)

	for _, match := range matches {
		r.pathVars = append(r.pathVars, match[1])
	}

	// Convert path pattern to regex
	// /orders/{id} -> ^/orders/([^/]+)$
	// /orders/* -> ^/orders/.*$
	regexPattern := pattern

	// Replace wildcards first (before escaping)
	regexPattern = strings.ReplaceAll(regexPattern, "*", "WILDCARD_PLACEHOLDER")

	// Escape special regex characters
	regexPattern = regexp.QuoteMeta(regexPattern)

	// Replace path variables with regex capture groups
	regexPattern = regexp.MustCompile(`\\{[^}]+\\}`).ReplaceAllString(regexPattern, `([^/]+)`)

	// Replace wildcard placeholder with .*
	regexPattern = strings.ReplaceAll(regexPattern, "WILDCARD_PLACEHOLDER", ".*")

	// Add anchors
	regexPattern = "^" + regexPattern + "$"

	var err error
	r.pathRegex, err = regexp.Compile(regexPattern)
	if err != nil {
		return fmt.Errorf("failed to compile path pattern %s: %w", pattern, err)
	}

	return nil
}

// Matches checks if the route matches the given path and method
func (r *Route) Matches(path, method string) bool {
	if r.Method != "" && !strings.EqualFold(r.Method, method) {
		return false
	}

	if r.pathRegex == nil {
		return false
	}

	return r.pathRegex.MatchString(path)
}

// ExtractPathVariables extracts path variables from the request path
func (r *Route) ExtractPathVariables(path string) map[string]string {
	if r.pathRegex == nil || len(r.pathVars) == 0 {
		return nil
	}

	matches := r.pathRegex.FindStringSubmatch(path)
	if len(matches) < 2 {
		return nil
	}

	variables := make(map[string]string)
	for i, varName := range r.pathVars {
		if i+1 < len(matches) {
			variables[varName] = matches[i+1]
		}
	}

	return variables
}

// GetTargetService returns the service name for this route
func (r *Route) GetTargetService() string {
	return r.Service
}

// GetGRPCTarget returns the gRPC service and method
func (r *Route) GetGRPCTarget() (service, method string) {
	return r.GRPCService, r.GRPCMethod
}

// RequiresAuth returns whether the route requires authentication
func (r *Route) RequiresAuth() bool {
	return r.AuthRequired
}

// String returns a string representation of the route
func (r *Route) String() string {
	auth := "public"
	if r.AuthRequired {
		auth = "protected"
	}
	return fmt.Sprintf("%s %s -> %s.%s (%s)", r.Method, r.Path, r.GRPCService, r.GRPCMethod, auth)
}
