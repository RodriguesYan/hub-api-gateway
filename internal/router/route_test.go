package router

import (
	"testing"
)

func TestRoute_CompilePathPattern(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		shouldError bool
	}{
		{
			name:        "exact path",
			path:        "/api/v1/orders",
			shouldError: false,
		},
		{
			name:        "path with single variable",
			path:        "/api/v1/orders/{id}",
			shouldError: false,
		},
		{
			name:        "path with multiple variables",
			path:        "/api/v1/orders/{id}/items/{itemId}",
			shouldError: false,
		},
		{
			name:        "path with wildcard",
			path:        "/api/v1/orders/*",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{Path: tt.path}
			err := route.CompilePathPattern()

			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.shouldError && route.pathRegex == nil {
				t.Errorf("pathRegex should not be nil")
			}
		})
	}
}

func TestRoute_Matches(t *testing.T) {
	tests := []struct {
		name        string
		routePath   string
		routeMethod string
		testPath    string
		testMethod  string
		shouldMatch bool
	}{
		{
			name:        "exact match",
			routePath:   "/api/v1/orders",
			routeMethod: "GET",
			testPath:    "/api/v1/orders",
			testMethod:  "GET",
			shouldMatch: true,
		},
		{
			name:        "path variable match",
			routePath:   "/api/v1/orders/{id}",
			routeMethod: "GET",
			testPath:    "/api/v1/orders/123",
			testMethod:  "GET",
			shouldMatch: true,
		},
		{
			name:        "wildcard match",
			routePath:   "/api/v1/orders/*",
			routeMethod: "GET",
			testPath:    "/api/v1/orders/123/items",
			testMethod:  "GET",
			shouldMatch: true,
		},
		{
			name:        "method mismatch",
			routePath:   "/api/v1/orders",
			routeMethod: "GET",
			testPath:    "/api/v1/orders",
			testMethod:  "POST",
			shouldMatch: false,
		},
		{
			name:        "path mismatch",
			routePath:   "/api/v1/orders",
			routeMethod: "GET",
			testPath:    "/api/v1/positions",
			testMethod:  "GET",
			shouldMatch: false,
		},
		{
			name:        "case insensitive method",
			routePath:   "/api/v1/orders",
			routeMethod: "GET",
			testPath:    "/api/v1/orders",
			testMethod:  "get",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{
				Path:   tt.routePath,
				Method: tt.routeMethod,
			}

			if err := route.CompilePathPattern(); err != nil {
				t.Fatalf("failed to compile pattern: %v", err)
			}

			matches := route.Matches(tt.testPath, tt.testMethod)
			if matches != tt.shouldMatch {
				t.Errorf("expected match=%v but got %v", tt.shouldMatch, matches)
			}
		})
	}
}

func TestRoute_ExtractPathVariables(t *testing.T) {
	tests := []struct {
		name      string
		routePath string
		testPath  string
		expected  map[string]string
	}{
		{
			name:      "single variable",
			routePath: "/api/v1/orders/{id}",
			testPath:  "/api/v1/orders/123",
			expected: map[string]string{
				"id": "123",
			},
		},
		{
			name:      "multiple variables",
			routePath: "/api/v1/orders/{orderId}/items/{itemId}",
			testPath:  "/api/v1/orders/123/items/456",
			expected: map[string]string{
				"orderId": "123",
				"itemId":  "456",
			},
		},
		{
			name:      "no variables",
			routePath: "/api/v1/orders",
			testPath:  "/api/v1/orders",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{Path: tt.routePath}

			if err := route.CompilePathPattern(); err != nil {
				t.Fatalf("failed to compile pattern: %v", err)
			}

			variables := route.ExtractPathVariables(tt.testPath)

			if tt.expected == nil && variables != nil {
				t.Errorf("expected nil but got %v", variables)
				return
			}

			if tt.expected != nil && variables == nil {
				t.Errorf("expected %v but got nil", tt.expected)
				return
			}

			if tt.expected != nil {
				for key, expectedValue := range tt.expected {
					if actualValue, ok := variables[key]; !ok {
						t.Errorf("missing variable %s", key)
					} else if actualValue != expectedValue {
						t.Errorf("variable %s: expected %s but got %s", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestRoute_RequiresAuth(t *testing.T) {
	tests := []struct {
		name         string
		authRequired bool
		expected     bool
	}{
		{
			name:         "requires auth",
			authRequired: true,
			expected:     true,
		},
		{
			name:         "public route",
			authRequired: false,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{AuthRequired: tt.authRequired}
			if route.RequiresAuth() != tt.expected {
				t.Errorf("expected %v but got %v", tt.expected, route.RequiresAuth())
			}
		})
	}
}

func TestRoute_GetGRPCTarget(t *testing.T) {
	route := &Route{
		GRPCService: "OrderService",
		GRPCMethod:  "SubmitOrder",
	}

	service, method := route.GetGRPCTarget()

	if service != "OrderService" {
		t.Errorf("expected service OrderService but got %s", service)
	}

	if method != "SubmitOrder" {
		t.Errorf("expected method SubmitOrder but got %s", method)
	}
}
