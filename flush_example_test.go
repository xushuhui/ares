package ares_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xushuhui/ares"
)

// TestContext_Flush demonstrates how to use the Flush method for streaming responses
func TestContext_Flush(t *testing.T) {
	app := ares.New()

	// Handler that sends data in chunks with flushing
	app.GET("/stream", func(ctx *ares.Context) error {
		// Set content type for streaming
		ctx.SetHeader("Content-Type", "text/plain")
		ctx.WriteHeader(http.StatusOK)

		// Send first chunk
		ctx.Write([]byte("First chunk of data\n"))
		if flushed := ctx.Flush(); !flushed {
			return fmt.Errorf("flush not supported")
		}

		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)

		// Send second chunk
		ctx.Write([]byte("Second chunk of data\n"))
		ctx.Flush()

		// Simulate more processing
		time.Sleep(100 * time.Millisecond)

		// Send final chunk
		ctx.Write([]byte("Final chunk\n"))
		ctx.Flush()

		return nil
	})

	// Test the streaming endpoint
	req := httptest.NewRequest("GET", "/stream", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
		return
	}

	body := rr.Body.String()
	expected := "First chunk of data\nSecond chunk of data\nFinal chunk\n"
	if body != expected {
		t.Errorf("Expected body:\n%s\nGot:\n%s\n", expected, body)
		return
	}

	fmt.Println("Streaming response successful!")
	fmt.Printf("Response body: %s", body)
}
