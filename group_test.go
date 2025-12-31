package ares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGroupBasic(t *testing.T) {
	app := New()

	api := app.Group("/api")
	api.GET("/users", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"message": "users"})
	})

	req := httptest.NewRequest("GET", "/api/users", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "users") {
		t.Error("Expected response to contain 'users'")
	}
}

func TestGroupMiddleware(t *testing.T) {
	app := New()

	// Middleware that adds a header
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Auth", "authenticated")
			next.ServeHTTP(w, r)
		})
	}

	api := app.Group("/api", authMiddleware)
	api.GET("/protected", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"message": "protected"})
	})

	req := httptest.NewRequest("GET", "/api/protected", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Header().Get("X-Auth") != "authenticated" {
		t.Error("Expected X-Auth header to be set by middleware")
	}
}

func TestGroupNestedGroups(t *testing.T) {
	app := New()

	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.GET("/users", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"version": "v1"})
	})

	v2 := api.Group("/v2")
	v2.GET("/users", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"version": "v2"})
	})

	// Test v1
	req1 := httptest.NewRequest("GET", "/api/v1/users", nil)
	rr1 := httptest.NewRecorder()
	app.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for v1, got %d", rr1.Code)
	}
	if !strings.Contains(rr1.Body.String(), "v1") {
		t.Error("Expected v1 response")
	}

	// Test v2
	req2 := httptest.NewRequest("GET", "/api/v2/users", nil)
	rr2 := httptest.NewRecorder()
	app.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for v2, got %d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), "v2") {
		t.Error("Expected v2 response")
	}
}

func TestGroupNestedMiddleware(t *testing.T) {
	app := New()

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-1", "applied")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-2", "applied")
			next.ServeHTTP(w, r)
		})
	}

	api := app.Group("/api", middleware1)
	v1 := api.Group("/v1", middleware2)
	v1.GET("/test", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Both middleware should be applied
	if rr.Header().Get("X-Middleware-1") != "applied" {
		t.Error("Expected middleware 1 to be applied")
	}
	if rr.Header().Get("X-Middleware-2") != "applied" {
		t.Error("Expected middleware 2 to be applied")
	}
}

func TestGroupHTTPMethods(t *testing.T) {
	app := New()

	api := app.Group("/api")
	api.GET("/resource", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"method": "GET"})
	})
	api.POST("/resource", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"method": "POST"})
	})
	api.PUT("/resource", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"method": "PUT"})
	})
	api.DELETE("/resource", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"method": "DELETE"})
	})
	api.PATCH("/resource", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"method": "PATCH"})
	})

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/resource", nil)
		rr := httptest.NewRecorder()

		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200 for %s, got %d", method, rr.Code)
		}

		if !strings.Contains(rr.Body.String(), method) {
			t.Errorf("Expected response to contain method %s", method)
		}
	}
}

func TestGroupUseMethod(t *testing.T) {
	app := New()

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-1", "1")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-2", "2")
			next.ServeHTTP(w, r)
		})
	}

	api := app.Group("/api")
	api.Use(middleware1, middleware2)
	api.GET("/test", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Header().Get("X-Test-1") != "1" {
		t.Error("Expected middleware 1 to be applied")
	}
	if rr.Header().Get("X-Test-2") != "2" {
		t.Error("Expected middleware 2 to be applied")
	}
}

func TestGroupWithParams(t *testing.T) {
	app := New()

	api := app.Group("/api")
	api.GET("/users/{id}", func(ctx *Context) error {
		id := ctx.Param("id")
		return ctx.JSON(http.StatusOK, map[string]string{"id": id})
	})

	req := httptest.NewRequest("GET", "/api/users/123", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "123") {
		t.Error("Expected response to contain user ID")
	}
}
