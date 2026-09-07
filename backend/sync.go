package main

import (
	"backend/proxies"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

var syncStatusMu sync.Mutex

type SyncOperation string

const (
	SyncOpCanvas       SyncOperation = "canvas"
	SyncOpGoogleImport SyncOperation = "google_import"
	SyncOpGoogleExport SyncOperation = "google_export"
)

// getOrCreateSyncStatus retrieves the 1:1 singleton sync_status record for a user or creates it if missing.
func getOrCreateSyncStatus(app core.App, userID string) (*proxies.SyncStatus, error) {
	record, err := app.FindFirstRecordByFilter(proxies.CollectionSyncStatus, "user = {:user}", map[string]any{
		"user": userID,
	})
	if err == nil && record != nil {
		return proxies.NewSyncStatus(record), nil
	}

	col, err := app.FindCollectionByNameOrId(proxies.CollectionSyncStatus)
	if err != nil {
		return nil, fmt.Errorf("sync_status collection not found: %w", err)
	}

	newRec := proxies.NewSyncStatusRecord(col)
	newRec.SetUserID(userID)
	newRec.SetCanvasStatus(proxies.SyncStatusStateIdle)
	newRec.SetGoogleImportStatus(proxies.SyncStatusStateIdle)
	newRec.SetGoogleExportStatus(proxies.SyncStatusStateIdle)
	newRec.SetGoogleExportEnabled(true)
	newRec.SetHistory([]proxies.SyncHistoryEntry{})

	if err := app.Save(newRec); err != nil {
		return nil, fmt.Errorf("failed to create sync_status record: %w", err)
	}
	return newRec, nil
}

// updateSyncStatusRunning marks an operation as "running" so connected frontends react immediately.
func updateSyncStatusRunning(app core.App, userID string, op SyncOperation) error {
	syncStatusMu.Lock()
	defer syncStatusMu.Unlock()

	rec, err := getOrCreateSyncStatus(app, userID)
	if err != nil {
		return err
	}

	switch op {
	case SyncOpCanvas:
		rec.SetCanvasStatus(proxies.SyncStatusStateRunning)
		rec.SetCanvasError("")
	case SyncOpGoogleImport:
		rec.SetGoogleImportStatus(proxies.SyncStatusStateRunning)
		rec.SetGoogleImportError("")
	case SyncOpGoogleExport:
		rec.SetGoogleExportStatus(proxies.SyncStatusStateRunning)
		rec.SetGoogleExportError("")
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
	syncStatusMu.Lock()
	defer syncStatusMu.Unlock()

	rec, err := getOrCreateSyncStatus(app, userID)
	if err != nil {
		return err
	}

	nowISO := time.Now().UTC().Format(time.RFC3339)

	switch op {
	case SyncOpCanvas:
		rec.SetCanvasStatus(proxies.SyncStatusStateIdle)
		if status == "success" {
			rec.SetCanvasSyncedAt(nowISO)
			rec.SetCanvasFeedback(feedback)
			rec.SetCanvasError("")
		} else {
			rec.SetCanvasError(errMsg)
		}
	case SyncOpGoogleImport:
		rec.SetGoogleImportStatus(proxies.SyncStatusStateIdle)
		if status == "success" {
			rec.SetGoogleImportSyncedAt(nowISO)
			rec.SetGoogleImportFeedback(feedback)
			rec.SetGoogleImportError("")
		} else {
			rec.SetGoogleImportError(errMsg)
		}
	case SyncOpGoogleExport:
		rec.SetGoogleExportStatus(proxies.SyncStatusStateIdle)
		if status == "success" {
			rec.SetGoogleExportSyncedAt(nowISO)
			rec.SetGoogleExportFeedback(feedback)
			rec.SetGoogleExportError("")
		} else {
			rec.SetGoogleExportError(errMsg)
		}
	}

	// Append to history log
	history := rec.History()

	entry := proxies.SyncHistoryEntry{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		Service:    proxies.SyncService(op),
		Status:     proxies.SyncResult(status),
		Timestamp:  nowISO,
		DurationMS: durationMs,
		Feedback:   feedback,
		Error:      errMsg,
	}

	// Prepend most recent entry and cap at 50
	history = append([]proxies.SyncHistoryEntry{entry}, history...)
	if len(history) > 50 {
		history = history[:50]
	}

	rec.SetHistory(history)

	if err := app.Save(rec); err != nil {
		log.Printf("Failed to save sync_status for user %s: %v", userID, err)
		return err
	}
	return nil
}
