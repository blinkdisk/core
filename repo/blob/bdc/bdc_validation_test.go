package bdc

import (
	"context"
	"testing"
)

func TestBdcStorageValidation(t *testing.T) {
	t.Run("MissingURL", func(t *testing.T) {
		opts := &Options{
			Token: "test-token",
		}
		_, err := New(context.Background(), opts, true)
		if err == nil {
			t.Error("Expected error for missing URL")
		}
	})

	t.Run("MissingToken", func(t *testing.T) {
		opts := &Options{
			URL: "ws://localhost:8080",
		}
		_, err := New(context.Background(), opts, true)
		if err == nil {
			t.Error("Expected error for missing token")
		}
	})

	t.Run("InvalidURL", func(t *testing.T) {
		opts := &Options{
			URL:   "not-a-url",
			Token: "test-token",
		}
		_, err := New(context.Background(), opts, true)
		if err == nil {
			t.Error("Expected error for invalid URL")
		}
	})
}
