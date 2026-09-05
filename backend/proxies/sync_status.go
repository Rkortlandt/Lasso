package proxies

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
)

// CollectionSyncStatus is the PocketBase collection name for sync_status.
const CollectionSyncStatus = "sync_status"

// SyncStatusState represents the current execution state of a sync operation.
type SyncStatusState string

const (
	SyncStatusStateIdle    SyncStatusState = "idle"
	SyncStatusStateRunning SyncStatusState = "running"
	SyncStatusStateSuccess SyncStatusState = "success"
	SyncStatusStateError   SyncStatusState = "error"
)

// SyncService represents a syncable third-party service or direction.
type SyncService string

const (
	SyncServiceCanvas       SyncService = "canvas"
	SyncServiceGoogleImport SyncService = "google_import"
	SyncServiceGoogleExport SyncService = "google_export"
)

// SyncResult represents the outcome status of a completed sync history entry.
type SyncResult string

const (
	SyncResultSuccess SyncResult = "success"
	SyncResultError   SyncResult = "error"
)

// Compile-time check ensuring SyncStatus implements core.RecordProxy
var _ core.RecordProxy = (*SyncStatus)(nil)

// SyncHistoryEntry represents a single sync audit entry stored in the history array.
type SyncHistoryEntry struct {
	ID         string      `json:"id"`
	Service    SyncService `json:"service"`
	Status     SyncResult  `json:"status"`
	Timestamp  string      `json:"timestamp"`
	DurationMS int64       `json:"duration_ms,omitempty"`
	Feedback   string      `json:"feedback,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// SyncStatus wraps a PocketBase Record from the "sync_status" collection.
type SyncStatus struct {
	core.BaseRecordProxy
}

// NewSyncStatus wraps an existing core.Record in a SyncStatus proxy.
func NewSyncStatus(record *core.Record) *SyncStatus {
	s := &SyncStatus{}
	s.SetProxyRecord(record)
	return s
}

// NewSyncStatusRecord creates a new record from the given collection and wraps it in a SyncStatus proxy.
func NewSyncStatusRecord(col *core.Collection) *SyncStatus {
	rec := core.NewRecord(col)
	s := &SyncStatus{}
	s.SetProxyRecord(rec)
	return s
}

// Relation: User

func (s *SyncStatus) UserID() string {
	return s.GetString("user")
}

func (s *SyncStatus) SetUserID(userID string) {
	s.Set("user", userID)
}

// User returns the expanded User proxy if loaded via PocketBase expand, or nil.
func (s *SyncStatus) User() *User {
	if rec := s.ExpandedOne("user"); rec != nil {
		return NewUser(rec)
	}
	return nil
}

// SetUser assigns the foreign key from a User proxy.
func (s *SyncStatus) SetUser(user *User) {
	if user != nil {
		s.SetUserID(user.Id)
	} else {
		s.SetUserID("")
	}
}

// Canvas Sync Fields

func (s *SyncStatus) CanvasStatus() SyncStatusState {
	return SyncStatusState(s.GetString("canvas_status"))
}

func (s *SyncStatus) SetCanvasStatus(status SyncStatusState) {
	s.Set("canvas_status", string(status))
}

func (s *SyncStatus) CanvasSyncedAt() string {
	return s.GetString("canvas_synced_at")
}

func (s *SyncStatus) SetCanvasSyncedAt(syncedAt string) {
	s.Set("canvas_synced_at", syncedAt)
}

func (s *SyncStatus) CanvasError() string {
	return s.GetString("canvas_error")
}

func (s *SyncStatus) SetCanvasError(errMsg string) {
	s.Set("canvas_error", errMsg)
}

func (s *SyncStatus) CanvasFeedback() string {
	return s.GetString("canvas_feedback")
}

func (s *SyncStatus) SetCanvasFeedback(feedback string) {
	s.Set("canvas_feedback", feedback)
}

// Google Import Fields

func (s *SyncStatus) GoogleImportStatus() SyncStatusState {
	return SyncStatusState(s.GetString("google_import_status"))
}

func (s *SyncStatus) SetGoogleImportStatus(status SyncStatusState) {
	s.Set("google_import_status", string(status))
}

func (s *SyncStatus) GoogleImportSyncedAt() string {
	return s.GetString("google_import_synced_at")
}

func (s *SyncStatus) SetGoogleImportSyncedAt(syncedAt string) {
	s.Set("google_import_synced_at", syncedAt)
}

func (s *SyncStatus) GoogleImportError() string {
	return s.GetString("google_import_error")
}

func (s *SyncStatus) SetGoogleImportError(errMsg string) {
	s.Set("google_import_error", errMsg)
}

func (s *SyncStatus) GoogleImportFeedback() string {
	return s.GetString("google_import_feedback")
}

func (s *SyncStatus) SetGoogleImportFeedback(feedback string) {
	s.Set("google_import_feedback", feedback)
}

// Google Export Fields

func (s *SyncStatus) GoogleExportStatus() SyncStatusState {
	return SyncStatusState(s.GetString("google_export_status"))
}

func (s *SyncStatus) SetGoogleExportStatus(status SyncStatusState) {
	s.Set("google_export_status", string(status))
}

func (s *SyncStatus) GoogleExportSyncedAt() string {
	return s.GetString("google_export_synced_at")
}

func (s *SyncStatus) SetGoogleExportSyncedAt(syncedAt string) {
	s.Set("google_export_synced_at", syncedAt)
}

func (s *SyncStatus) GoogleExportError() string {
	return s.GetString("google_export_error")
}

func (s *SyncStatus) SetGoogleExportError(errMsg string) {
	s.Set("google_export_error", errMsg)
}

func (s *SyncStatus) GoogleExportFeedback() string {
	return s.GetString("google_export_feedback")
}

func (s *SyncStatus) SetGoogleExportFeedback(feedback string) {
	s.Set("google_export_feedback", feedback)
}

func (s *SyncStatus) GoogleExportEnabled() bool {
	return s.GetBool("google_export_enabled")
}

func (s *SyncStatus) SetGoogleExportEnabled(enabled bool) {
	s.Set("google_export_enabled", enabled)
}

// History Log Field

func (s *SyncStatus) History() []SyncHistoryEntry {
	raw := s.Get("history")
	if raw == nil {
		return []SyncHistoryEntry{}
	}

	var history []SyncHistoryEntry
	switch h := raw.(type) {
	case []SyncHistoryEntry:
		return h
	case string:
		_ = json.Unmarshal([]byte(h), &history)
	default:
		bytes, err := json.Marshal(h)
		if err == nil {
			_ = json.Unmarshal(bytes, &history)
		}
	}
	if history == nil {
		return []SyncHistoryEntry{}
	}
	return history
}

func (s *SyncStatus) SetHistory(history []SyncHistoryEntry) {
	s.Set("history", history)
}
