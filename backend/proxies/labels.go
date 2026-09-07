package proxies

import (
	"github.com/pocketbase/pocketbase/core"
)

// CollectionLabels is the PocketBase collection name for labels.
const CollectionLabels = "labels"

// Compile-time check ensuring Label implements core.RecordProxy
var _ core.RecordProxy = (*Label)(nil)

// Label wraps a PocketBase Record from the "labels" collection.
type Label struct {
	core.BaseRecordProxy
}

// NewLabel wraps an existing core.Record in a Label proxy.
func NewLabel(record *core.Record) *Label {
	l := &Label{}
	l.SetProxyRecord(record)
	return l
}

// NewLabelRecord creates a new record from the given collection and wraps it in a Label proxy.
func NewLabelRecord(col *core.Collection) *Label {
	rec := core.NewRecord(col)
	l := &Label{}
	l.SetProxyRecord(rec)
	return l
}

// Relation: User

func (l *Label) UserID() string {
	return l.GetString("user")
}

func (l *Label) SetUserID(userID string) {
	l.Set("user", userID)
}

// User returns the expanded User proxy if loaded via PocketBase expand, or nil.
func (l *Label) User() *User {
	if rec := l.ExpandedOne("user"); rec != nil {
		return NewUser(rec)
	}
	return nil
}

// SetUser assigns the foreign key from a User proxy.
func (l *Label) SetUser(user *User) {
	if user != nil {
		l.SetUserID(user.Id)
	} else {
		l.SetUserID("")
	}
}

// Field Getters and Setters

func (l *Label) Name() string {
	return l.GetString("name")
}

func (l *Label) SetName(name string) {
	l.Set("name", name)
}

func (l *Label) Color() string {
	return l.GetString("color")
}

func (l *Label) SetColor(color string) {
	l.Set("color", color)
}
