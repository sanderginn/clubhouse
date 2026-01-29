package services

import "testing"

func TestPushFailureTypeForStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantType   string
		wantFail   bool
	}{
		{"not_found", 404, "subscription_gone", true},
		{"gone", 410, "subscription_gone", true},
		{"bad_request", 400, "http_error", true},
		{"server_error", 503, "http_error", true},
		{"success", 201, "", false},
		{"no_status", 0, "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotType, gotFail := pushFailureTypeForStatus(tt.statusCode)
			if gotType != tt.wantType {
				t.Fatalf("expected type %q, got %q", tt.wantType, gotType)
			}
			if gotFail != tt.wantFail {
				t.Fatalf("expected failure %v, got %v", tt.wantFail, gotFail)
			}
		})
	}
}
