package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type SyncOperation string

const (
	SyncOpCanvas       SyncOperation = "canvas"
	SyncOpGoogleImport SyncOperation = "google_import"
	SyncOpGoogleExport SyncOperation = "google_export"
)

type SyncHistoryLogEntry struct {
	ID         string `json:"id"`
	Service    string `json:"service"`
	Status     string `json:"status"`
	Timestamp  string `json:"timestamp"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Feedback   string `json:"feedback,omitempty"`
	Error      string `json:"error,omitempty"`
}

// getOrCreateSyncStatus retrieves the 1:1 singleton sync_status record for a user or creates it if missing.
func getOrCreateSyncStatus(app core.App, userID string) (*core.Record, error) {
	record, err := app.FindFirstRecordByFilter("sync_status", "user = {:user}", map[string]any{
		"user": userID,
	})
	if err == nil && record != nil {
		return record, nil
	}

	col, err := app.FindCollectionByNameOrId("sync_status")
	if err != nil {
		return nil, fmt.Errorf("sync_status collection not found: %w", err)
	}

	newRec := core.NewRecord(col)
	newRec.Set("user", userID)
	newRec.Set("canvas_status", "idle")
	newRec.Set("google_import_status", "idle")
	newRec.Set("google_export_status", "idle")
	newRec.Set("google_export_enabled", true)
	newRec.Set("history", []any{})

	if err := app.Save(newRec); err != nil {
		return nil, fmt.Errorf("failed to create sync_status record: %w", err)
	}
	return newRec, nil
}

// updateSyncStatusRunning marks an operation as "running" so connected frontends react immediately.
func updateSyncStatusRunning(app core.App, userID string, op SyncOperation) error {
	rec, err := getOrCreateSyncStatus(app, userID)
	if err != nil {
		return err
	}

	switch op {
	case SyncOpCanvas:
		rec.Set("canvas_status", "running")
		rec.Set("canvas_error", "")
	case SyncOpGoogleImport:
		rec.Set("google_import_status", "running")
		rec.Set("google_import_error", "")
	case SyncOpGoogleExport:
		rec.Set("google_export_status", "running")
		rec.Set("google_export_error", "")
	}

	return app.Save(rec)
}

// updateSyncStatusFinished marks an operation as finished ("success" or "error"), records timestamps, feedback, and history.
func updateSyncStatusFinished(
	app core.App,
	userID string,
	op SyncOperation,
	status string,
	feedback string,
	errMsg string,
	durationMs int64,
) error {
	rec, err := getOrCreateSyncStatus(app, userID)
	if err != nil {
		return err
	}

	nowISO := time.Now().UTC().Format(time.RFC3339)

	switch op {
	case SyncOpCanvas:
		rec.Set("canvas_status", "idle")
		if status == "success" {
			rec.Set("canvas_synced_at", nowISO)
			rec.Set("canvas_feedback", feedback)
			rec.Set("canvas_error", "")
		} else {
			rec.Set("canvas_error", errMsg)
		}
	case SyncOpGoogleImport:
		rec.Set("google_import_status", "idle")
		if status == "success" {
			rec.Set("google_import_synced_at", nowISO)
			rec.Set("google_import_feedback", feedback)
			rec.Set("google_import_error", "")
		} else {
			rec.Set("google_import_error", errMsg)
		}
	case SyncOpGoogleExport:
		rec.Set("google_export_status", "idle")
		if status == "success" {
			rec.Set("google_export_synced_at", nowISO)
			rec.Set("google_export_feedback", feedback)
			rec.Set("google_export_error", "")
		} else {
			rec.Set("google_export_error", errMsg)
		}
	}

	// Append to history log
	var history []SyncHistoryLogEntry
	historyRaw := rec.Get("history")
	if historyRaw != nil {
		switch h := historyRaw.(type) {
		case string:
			_ = json.Unmarshal([]byte(h), &history)
		default:
			bytes, err := json.Marshal(h)
			if err == nil {
				_ = json.Unmarshal(bytes, &history)
			}
		}
	}

	entry := SyncHistoryLogEntry{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		Service:    string(op),
		Status:     status,
		Timestamp:  nowISO,
		DurationMS: durationMs,
		Feedback:   feedback,
		Error:      errMsg,
	}

	// Prepend most recent entry and cap at 50
	history = append([]SyncHistoryLogEntry{entry}, history...)
	if len(history) > 50 {
		history = history[:50]
	}

	rec.Set("history", history)

	if err := app.Save(rec); err != nil {
		log.Printf("Failed to save sync_status for user %s: %v", userID, err)
		return err
	}
	return nil
}
