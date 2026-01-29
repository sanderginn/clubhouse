package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestLogAuditWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	adminID := testutil.CreateTestUser(t, db, "auditadmin", "auditadmin@test.com", true, true)
	targetID := testutil.CreateTestUser(t, db, "audittarget", "audittarget@test.com", false, true)

	metadata := map[string]interface{}{
		"reason": "spam",
		"count":  3,
	}

	service := NewAuditService(db)
	err := service.LogAuditWithMetadata(
		context.Background(),
		"suspend_user",
		uuid.MustParse(adminID),
		uuid.MustParse(targetID),
		metadata,
	)
	if err != nil {
		t.Fatalf("LogAuditWithMetadata failed: %v", err)
	}

	var storedTargetID uuid.UUID
	var storedRelatedID uuid.UUID
	var metadataBytes []byte
	err = db.QueryRowContext(
		context.Background(),
		`SELECT target_user_id, related_user_id, metadata FROM audit_logs WHERE admin_user_id = $1 AND action = 'suspend_user'`,
		uuid.MustParse(adminID),
	).Scan(&storedTargetID, &storedRelatedID, &metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	if storedTargetID.String() != targetID {
		t.Errorf("expected target_user_id %s, got %s", targetID, storedTargetID.String())
	}
	if storedRelatedID.String() != targetID {
		t.Errorf("expected related_user_id %s, got %s", targetID, storedRelatedID.String())
	}

	var storedMetadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &storedMetadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if storedMetadata["reason"] != "spam" {
		t.Errorf("expected metadata reason 'spam', got %v", storedMetadata["reason"])
	}
	if storedMetadata["count"] != float64(3) {
		t.Errorf("expected metadata count 3, got %v", storedMetadata["count"])
	}
}
