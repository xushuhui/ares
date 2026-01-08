package errors

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *Error
		want    string
	}{
		{
			name: "Error with code and message",
			err: &Error{
				Code:    404,
				Message: "Not Found",
			},
			want: "code=404, message=Not Found",
		},
		{
			name: "Error with different code",
			err: &Error{
				Code:    500,
				Message: "Internal Server Error",
			},
			want: "code=500, message=Internal Server Error",
		},
		{
			name: "Error with empty message",
			err: &Error{
				Code:    200,
				Message: "",
			},
			want: "code=200, message=",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		message string
	}{
		{
			name:    "Create error with code 400",
			code:    400,
			message: "Bad Request",
		},
		{
			name:    "Create error with code 404",
			code:    404,
			message: "Not Found",
		},
		{
			name:    "Create error with code 500",
			code:    500,
			message: "Internal Server Error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, tt.message)
			if err == nil {
				t.Fatal("New() returned nil")
			}

			customErr, ok := err.(*Error)
			if !ok {
				t.Fatal("New() did not return *Error type")
			}

			if customErr.Code != tt.code {
				t.Errorf("New() Code = %v, want %v", customErr.Code, tt.code)
			}

			if customErr.Message != tt.message {
				t.Errorf("New() Message = %v, want %v", customErr.Message, tt.message)
			}
		})
	}
}

func TestBadRequest(t *testing.T) {
	message := "Invalid input"
	err := BadRequest(message)

	if err == nil {
		t.Fatal("BadRequest() returned nil")
	}

	customErr, ok := err.(*Error)
	if !ok {
		t.Fatal("BadRequest() did not return *Error type")
	}

	if customErr.Code != http.StatusBadRequest {
		t.Errorf("BadRequest() Code = %v, want %v", customErr.Code, http.StatusBadRequest)
	}

	if customErr.Message != message {
		t.Errorf("BadRequest() Message = %v, want %v", customErr.Message, message)
	}
}

func TestNotFound(t *testing.T) {
	message := "Resource not found"
	err := NotFound(message)

	if err == nil {
		t.Fatal("NotFound() returned nil")
	}

	customErr, ok := err.(*Error)
	if !ok {
		t.Fatal("NotFound() did not return *Error type")
	}

	if customErr.Code != http.StatusNotFound {
		t.Errorf("NotFound() Code = %v, want %v", customErr.Code, http.StatusNotFound)
	}

	if customErr.Message != message {
		t.Errorf("NotFound() Message = %v, want %v", customErr.Message, message)
	}
}

func TestInternalError(t *testing.T) {
	message := "Something went wrong"
	err := InternalError(message)

	if err == nil {
		t.Fatal("InternalError() returned nil")
	}

	customErr, ok := err.(*Error)
	if !ok {
		t.Fatal("InternalError() did not return *Error type")
	}

	if customErr.Code != http.StatusInternalServerError {
		t.Errorf("InternalError() Code = %v, want %v", customErr.Code, http.StatusInternalServerError)
	}

	if customErr.Message != message {
		t.Errorf("InternalError() Message = %v, want %v", customErr.Message, message)
	}
}

func TestForbidden(t *testing.T) {
	message := "Access denied"
	err := Forbidden(message)

	if err == nil {
		t.Fatal("Forbidden() returned nil")
	}

	customErr, ok := err.(*Error)
	if !ok {
		t.Fatal("Forbidden() did not return *Error type")
	}

	if customErr.Code != http.StatusForbidden {
		t.Errorf("Forbidden() Code = %v, want %v", customErr.Code, http.StatusForbidden)
	}

	if customErr.Message != message {
		t.Errorf("Forbidden() Message = %v, want %v", customErr.Message, message)
	}
}

func TestUnauthorized(t *testing.T) {
	message := "Authentication required"
	err := Unauthorized(message)

	if err == nil {
		t.Fatal("Unauthorized() returned nil")
	}

	customErr, ok := err.(*Error)
	if !ok {
		t.Fatal("Unauthorized() did not return *Error type")
	}

	if customErr.Code != http.StatusUnauthorized {
		t.Errorf("Unauthorized() Code = %v, want %v", customErr.Code, http.StatusUnauthorized)
	}

	if customErr.Message != message {
		t.Errorf("Unauthorized() Message = %v, want %v", customErr.Message, message)
	}
}

func TestError_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name      string
		err       *Error
		wantJSON  string
	}{
		{
			name: "Marshal error to JSON",
			err: &Error{
				Code:    404,
				Message: "Not Found",
			},
			wantJSON: `{"code":404,"message":"Not Found"}`,
		},
		{
			name: "Marshal error with special characters",
			err: &Error{
				Code:    400,
				Message: "Invalid \"input\"",
			},
			wantJSON: `{"code":400,"message":"Invalid \"input\""}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			if got := string(jsonData); got != tt.wantJSON {
				t.Errorf("json.Marshal() = %v, want %v", got, tt.wantJSON)
			}
		})
	}
}

func TestError_JSONUnmarshaling(t *testing.T) {
	jsonStr := `{"code":404,"message":"Not Found"}`

	var err Error
	if err := json.Unmarshal([]byte(jsonStr), &err); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if err.Code != 404 {
		t.Errorf("Unmarshaled Code = %v, want %v", err.Code, 404)
	}

	if err.Message != "Not Found" {
		t.Errorf("Unmarshaled Message = %v, want %v", err.Message, "Not Found")
	}
}

func TestError_Interface(t *testing.T) {
	// Test that Error implements error interface
	var _ error = &Error{}

	err := New(500, "test error")

	// Test that it satisfies the error interface
	if err.Error() == "" {
		t.Error("Error() returned empty string")
	}

	// Test type assertion
	customErr, ok := err.(*Error)
	if !ok {
		t.Fatal("Type assertion to *Error failed")
	}

	if customErr.Code != 500 {
		t.Errorf("Code = %v, want 500", customErr.Code)
	}

	if customErr.Message != "test error" {
		t.Errorf("Message = %v, want 'test error'", customErr.Message)
	}
}

func TestError_StandardHTTPStatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		constructor    func(string) error
		expectedCode   int
	}{
		{
			name:           "BadRequest",
			constructor:    BadRequest,
			expectedCode:   http.StatusBadRequest,
		},
		{
			name:           "NotFound",
			constructor:    NotFound,
			expectedCode:   http.StatusNotFound,
		},
		{
			name:           "InternalError",
			constructor:    InternalError,
			expectedCode:   http.StatusInternalServerError,
		},
		{
			name:           "Forbidden",
			constructor:    Forbidden,
			expectedCode:   http.StatusForbidden,
		},
		{
			name:           "Unauthorized",
			constructor:    Unauthorized,
			expectedCode:   http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor("test message")

			customErr, ok := err.(*Error)
			if !ok {
				t.Fatal("Error is not of type *Error")
			}

			if customErr.Code != tt.expectedCode {
				t.Errorf("Code = %v, want %v", customErr.Code, tt.expectedCode)
			}
		})
	}
}

func TestError_EmptyMessage(t *testing.T) {
	err := New(200, "")

	if err == nil {
		t.Fatal("New() with empty message returned nil")
	}

	customErr, ok := err.(*Error)
	if !ok {
		t.Fatal("New() did not return *Error type")
	}

	if customErr.Message != "" {
		t.Errorf("Message = %v, want empty string", customErr.Message)
	}
}

func TestError_NilError(t *testing.T) {
	// Test that nil error behaves correctly
	var err *Error
	if err != nil {
		t.Error("Expected nil *Error to be nil")
	}
}

func TestError_ErrorStringConsistency(t *testing.T) {
	// Test that Error() method produces consistent output
	err := &Error{
		Code:    404,
		Message: "Resource not found",
	}

	errorStr := err.Error()
	expected := "code=404, message=Resource not found"

	if errorStr != expected {
		t.Errorf("Error() = %v, want %v", errorStr, expected)
	}

	// Test consistency across multiple calls
	if err.Error() != errorStr {
		t.Error("Error() method returned different strings on multiple calls")
	}
}
