package proxies

import (
	"github.com/pocketbase/pocketbase/core"
)

// CollectionSyncRegistry is the PocketBase collection name for google_sync_registry.
const CollectionSyncRegistry = "google_sync_registry"

// SyncRegistryEntityType represents valid values for entity_type.
type SyncRegistryEntityType string

const (
	SyncRegistryEntityTypeTask  SyncRegistryEntityType = "task"
	SyncRegistryEntityTypeEvent SyncRegistryEntityType = "event"
)

// SyncRegistryStatus represents valid values for status.
type SyncRegistryStatus string

const (
	SyncRegistryStatusActive        SyncRegistryStatus = "active"
	SyncRegistryStatusPendingDelete SyncRegistryStatus = "pending_delete"
	SyncRegistryStatusDeleted       SyncRegistryStatus = "deleted"
)

// Compile-time check ensuring SyncRegistry implements core.RecordProxy
var _ core.RecordProxy = (*SyncRegistry)(nil)

// SyncRegistry wraps a PocketBase Record from the "google_sync_registry" collection.
type SyncRegistry struct {
	core.BaseRecordProxy
}

// NewSyncRegistry wraps an existing core.Record in a SyncRegistry proxy.
func NewSyncRegistry(record *core.Record) *SyncRegistry {
	s := &SyncRegistry{}
	s.SetProxyRecord(record)
	return s
}

// NewSyncRegistryRecord creates a new record from the given collection and wraps it in a SyncRegistry proxy.
func NewSyncRegistryRecord(col *core.Collection) *SyncRegistry {
	rec := core.NewRecord(col)
	s := &SyncRegistry{}
	s.SetProxyRecord(rec)
	return s
}

// Relation: User

func (s *SyncRegistry) UserID() string {
	return s.GetString("user")
}

func (s *SyncRegistry) SetUserID(userID string) {
	s.Set("user", userID)
}

func (s *SyncRegistry) User() *User {
	if rec := s.ExpandedOne("user"); rec != nil {
		return NewUser(rec)
	}
	return nil
}

func (s *SyncRegistry) SetUser(user *User) {
	if user != nil {
		s.SetUserID(user.Id)
	} else {
		s.SetUserID("")
	}
}

// Field Getters and Setters

func (s *SyncRegistry) EntityID() string {
	return s.GetString("entity_id")
}

func (s *SyncRegistry) SetEntityID(entityID string) {
	s.Set("entity_id", entityID)
}

func (s *SyncRegistry) EntityType() SyncRegistryEntityType {
	return SyncRegistryEntityType(s.GetString("entity_type"))
}

func (s *SyncRegistry) SetEntityType(entityType SyncRegistryEntityType) {
	s.Set("entity_type", string(entityType))
}

func (s *SyncRegistry) GoogleCalendarID() string {
	return s.GetString("google_calendar_id")
}

func (s *SyncRegistry) SetGoogleCalendarID(calID string) {
	s.Set("google_calendar_id", calID)
}

func (s *SyncRegistry) GoogleEventID() string {
	return s.GetString("google_event_id")
}

func (s *SyncRegistry) SetGoogleEventID(eventID string) {
	s.Set("google_event_id", eventID)
}

func (s *SyncRegistry) SyncedAt() string {
	return s.GetString("synced_at")
}

func (s *SyncRegistry) SetSyncedAt(syncedAt string) {
	s.Set("synced_at", syncedAt)
}

func (s *SyncRegistry) Status() SyncRegistryStatus {
	return SyncRegistryStatus(s.GetString("status"))
}

func (s *SyncRegistry) SetStatus(status SyncRegistryStatus) {
	s.Set("status", string(status))
}
