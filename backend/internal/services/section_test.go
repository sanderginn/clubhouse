package services

import (
	"context"
	"testing"

	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestSectionServiceNilDB(t *testing.T) {
	// Test that NewSectionService with nil db doesn't panic at creation time
	// (actual calls will panic, but that's expected - nil db is programmer error)
	service := NewSectionService(nil)
	if service == nil {
		t.Error("expected non-nil service even with nil db")
	}
}

func TestSectionServiceListSections(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test section
	testutil.CreateTestSection(t, db, "Music", "music")

	service := NewSectionService(db)
	sections, err := service.ListSections(context.Background())
	if err != nil {
		t.Fatalf("ListSections failed: %v", err)
	}

	if len(sections) == 0 {
		t.Error("expected at least one section")
	}
}
