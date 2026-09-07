package main

import (
	"backend/proxies"
	"log"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// recoverInterruptedSyncs queries sync_status for any user records left in the "running" state
// (due to an abrupt server shutdown or crash) and automatically resumes those syncs.
func recoverInterruptedSyncs(app core.App) {
	staleRecords, err := app.FindRecordsByFilter(
		proxies.CollectionSyncStatus,
		"canvas_status = 'running' || google_import_status = 'running' || google_export_status = 'running'",
		"",
		100,
		0,
	)
	if err != nil {
		log.Printf("[SyncRecovery] Error checking for stale sync records: %v", err)
		return
	}

	if len(staleRecords) == 0 {
		return
	}

	log.Printf("[SyncRecovery] Found %d sync record(s) left in 'running' state from previous shutdown. Starting auto-recovery...", len(staleRecords))

	for _, rec := range staleRecords {
		syncStatus := proxies.NewSyncStatus(rec)
		userID := syncStatus.UserID()
		if userID == "" {
			continue
		}

		userRec, errUser := app.FindRecordById("users", userID)
		if errUser != nil || userRec == nil {
			log.Printf("[SyncRecovery] User %s not found for sync record %s. Resetting status to idle.", userID, rec.Id)
			syncStatus.SetCanvasStatus(proxies.SyncStatusStateIdle)
			syncStatus.SetGoogleImportStatus(proxies.SyncStatusStateIdle)
			syncStatus.SetGoogleExportStatus(proxies.SyncStatusStateIdle)
			_ = app.Save(syncStatus)
			continue
		}

		userEmail := userRec.GetString("email")

		// 1. Recover Canvas sync if interrupted
		if syncStatus.CanvasStatus() == proxies.SyncStatusStateRunning {
			log.Printf("[SyncRecovery] Auto-resuming interrupted Canvas sync for user %s (%s)...", userEmail, userID)
			go func(u *core.Record) {
				_, err := runCanvasSyncForUser(app, u)
				if err != nil {
					log.Printf("[SyncRecovery] Canvas sync recovery for user %s failed: %v", u.Id, err)
				} else {
					log.Printf("[SyncRecovery] Successfully completed Canvas sync recovery for user %s", u.Id)
				}
			}(userRec)
		}

		// 2. Recover Google Inbound sync if interrupted
		if syncStatus.GoogleImportStatus() == proxies.SyncStatusStateRunning {
			log.Printf("[SyncRecovery] Auto-resuming interrupted Google Inbound sync for user %s (%s)...", userEmail, userID)
			go func(u *core.Record) {
				startTime := time.Now()
				res, err := runInboundGoogleSync(app, u)
				if err != nil {
					_ = updateSyncStatusFinished(app, u.Id, SyncOpGoogleImport, "error", "", err.Error(), time.Since(startTime).Milliseconds())
					log.Printf("[SyncRecovery] Google Inbound sync recovery for user %s failed: %v", u.Id, err)
				} else {
					_ = updateSyncStatusFinished(app, u.Id, SyncOpGoogleImport, "success", res.Message, "", time.Since(startTime).Milliseconds())
					log.Printf("[SyncRecovery] Successfully completed Google Inbound sync recovery for user %s", u.Id)
				}
			}(userRec)
		}

		// 3. Recover Google Outbound sync if interrupted
		if syncStatus.GoogleExportStatus() == proxies.SyncStatusStateRunning {
			log.Printf("[SyncRecovery] Auto-resuming interrupted Google Outbound sync for user %s (%s)...", userEmail, userID)
			go func(u *core.Record) {
				_, err := runOutboundGoogleSync(app, u)
				if err != nil {
					log.Printf("[SyncRecovery] Google Outbound sync recovery for user %s failed: %v", u.Id, err)
				} else {
					log.Printf("[SyncRecovery] Successfully completed Google Outbound sync recovery for user %s", u.Id)
				}
			}(userRec)
		}
	}
}
