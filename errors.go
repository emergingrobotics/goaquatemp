package goaquatemp

import (
	"errors"
	"fmt"
)

// Sentinel errors for common error conditions.
var (
	ErrNotAuthenticated   = errors.New("not authenticated")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrDeviceNotFound     = errors.New("device not found")
	ErrDeviceOffline      = errors.New("device offline")
	ErrInvalidTemperature = errors.New("temperature out of valid range")
	ErrInvalidMode        = errors.New("invalid operation mode")
)

// AuthError represents an authentication-related error.
type AuthError struct {
	Code      int
	Message   string
	Retryable bool
	Err       error
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("auth error %d: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("auth error %d: %s", e.Code, e.Message)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// APIError represents an error returned by the Aqua Temp API.
type APIError struct {
	StatusCode int
	ErrorCode  int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d (http %d): %s", e.ErrorCode, e.StatusCode, e.Message)
}

// DeviceError represents an error related to a specific device operation.
type DeviceError struct {
	DeviceCode string
	Operation  string
	Err        error
}

func (e *DeviceError) Error() string {
	return fmt.Sprintf("device %s: %s failed: %v", e.DeviceCode, e.Operation, e.Err)
}

func (e *DeviceError) Unwrap() error {
	return e.Err
}

// ValidationError represents a validation error for input parameters.
type ValidationError struct {
	Field   string
	Value   any
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s=%v: %s", e.Field, e.Value, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// IsAuthError returns true if the error is an authentication error.
func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

// IsAPIError returns true if the error is an API error.
func IsAPIError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr)
}

// IsDeviceError returns true if the error is a device error.
func IsDeviceError(err error) bool {
	var deviceErr *DeviceError
	return errors.As(err, &deviceErr)
}

// IsRetryable returns true if the error is retryable.
func IsRetryable(err error) bool {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr.Retryable
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// Retry on server errors
		return apiErr.StatusCode >= 500
	}

	return false
}
