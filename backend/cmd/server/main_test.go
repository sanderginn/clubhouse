package main

import "testing"

func TestGetEnvInt(t *testing.T) {
	t.Run("uses default when not set", func(t *testing.T) {
		t.Setenv("TEST_ENV_INT", "")
		if got := getEnvInt("TEST_ENV_INT", 3); got != 3 {
			t.Fatalf("expected 3, got %d", got)
		}
	})

	t.Run("uses env value when set", func(t *testing.T) {
		t.Setenv("TEST_ENV_INT", "5")
		if got := getEnvInt("TEST_ENV_INT", 3); got != 5 {
			t.Fatalf("expected 5, got %d", got)
		}
	})

	t.Run("uses default on invalid value", func(t *testing.T) {
		t.Setenv("TEST_ENV_INT", "abc")
		if got := getEnvInt("TEST_ENV_INT", 3); got != 3 {
			t.Fatalf("expected 3, got %d", got)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Setenv("TEST_ENV_INT", " 7 ")
		if got := getEnvInt("TEST_ENV_INT", 3); got != 7 {
			t.Fatalf("expected 7, got %d", got)
		}
	})
}
