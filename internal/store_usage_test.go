package buzzhive

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUsageStatsRollupAndSummary(t *testing.T) {
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	base := time.Date(2026, 6, 5, 9, 13, 0, 0, time.Local)
	records := []UsageRecord{
		{UserID: 1, UserName: "alice", UserAPIKeyID: 10, UserAPIKeyName: "k1", Model: "mimo-v2.5", Status: 200, LatencyMS: 100, CreatedAt: base},
		{UserID: 1, UserName: "alice", UserAPIKeyID: 10, UserAPIKeyName: "k1", Model: "mimo-v2.5", Status: 500, LatencyMS: 300, CreatedAt: base.Add(27 * time.Minute)},
		{UserID: 1, UserName: "alice", UserAPIKeyID: 11, UserAPIKeyName: "k2", Model: "mimo-v2.5", Status: 200, LatencyMS: 500, CreatedAt: base.Add(57 * time.Minute)},
	}
	if err := store.InsertUsageBatch(records); err != nil {
		t.Fatal(err)
	}

	var hourlyRows int
	if err := store.queryRow(`SELECT COUNT(1) FROM usage_stats_hourly`).Scan(&hourlyRows); err != nil {
		t.Fatal(err)
	}
	if hourlyRows != 2 {
		t.Fatalf("hourly rows = %d, want 2", hourlyRows)
	}

	shortSummary, err := store.UsageSummary(UsageQuery{
		UserID: 1,
		From:   time.Date(2026, 6, 5, 9, 0, 0, 0, time.Local),
		To:     time.Date(2026, 6, 5, 10, 0, 0, 0, time.Local),
	})
	if err != nil {
		t.Fatal(err)
	}
	if shortSummary.BucketMinutes != 1 || shortSummary.Requests != 2 || shortSummary.Errors != 1 {
		t.Fatalf("short summary = %+v, want 1m logs with 2 requests and 1 error", shortSummary)
	}

	longSummary, err := store.UsageSummary(UsageQuery{
		UserID: 1,
		From:   time.Date(2026, 6, 5, 9, 0, 0, 0, time.Local),
		To:     time.Date(2026, 6, 5, 12, 0, 0, 0, time.Local),
	})
	if err != nil {
		t.Fatal(err)
	}
	if longSummary.BucketMinutes != 60 || longSummary.Requests != 3 || longSummary.Errors != 1 {
		t.Fatalf("long summary = %+v, want hourly stats with 3 requests and 1 error", longSummary)
	}
	if got := longSummary.ByKey["k1"]; got != 2 {
		t.Fatalf("k1 usage = %d, want 2", got)
	}
	if len(longSummary.Series) != 2 {
		t.Fatalf("series len = %d, want 2: %+v", len(longSummary.Series), longSummary.Series)
	}
	if longSummary.Series[0].Date != "2026-06-05T09:00" || longSummary.Series[0].Requests != 2 || longSummary.Series[0].Errors != 1 {
		t.Fatalf("first hourly point = %+v", longSummary.Series[0])
	}
}
