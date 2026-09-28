package proxies

import (
	"github.com/pocketbase/pocketbase/core"
)

// CollectionEvents is the PocketBase collection name for events.
const CollectionEvents = "events"

// Compile-time check ensuring Event implements core.RecordProxy
var _ core.RecordProxy = (*Event)(nil)

// Event wraps a PocketBase Record from the "events" collection.
type Event struct {
	core.BaseRecordProxy
}

// NewEvent wraps an existing core.Record in an Event proxy.
func NewEvent(record *core.Record) *Event {
	e := &Event{}
	e.SetProxyRecord(record)
	return e
}

// NewEventRecord creates a new record from the given collection and wraps it in an Event proxy.
func NewEventRecord(col *core.Collection) *Event {
	rec := core.NewRecord(col)
	e := &Event{}
	e.SetProxyRecord(rec)
	return e
}

// Relation: Calendar

func (e *Event) CalendarID() string {
	return e.GetString("calendar")
}

func (e *Event) SetCalendarID(calendarID string) {
	e.Set("calendar", calendarID)
}

// Calendar returns the expanded Calendar proxy if loaded via PocketBase expand, or nil.
func (e *Event) Calendar() *Calendar {
	if rec := e.ExpandedOne("calendar"); rec != nil {
		return NewCalendar(rec)
	}
	return nil
}

// SetCalendar assigns the foreign key from a Calendar proxy.
func (e *Event) SetCalendar(cal *Calendar) {
	if cal != nil {
		e.SetCalendarID(cal.Id)
	} else {
		e.SetCalendarID("")
	}
}

// Relation: Task

func (e *Event) TaskID() string {
	return e.GetString("task")
}

func (e *Event) SetTaskID(taskID string) {
	e.Set("task", taskID)
}

// Task returns the expanded Task proxy if loaded via PocketBase expand, or nil.
func (e *Event) Task() *Task {
	if rec := e.ExpandedOne("task"); rec != nil {
		return NewTask(rec)
	}
	return nil
}

// SetTask assigns the foreign key from a Task proxy.
func (e *Event) SetTask(task *Task) {
	if task != nil {
		e.SetTaskID(task.Id)
	} else {
		e.SetTaskID("")
	}
}

// Field Getters and Setters

func (e *Event) Title() string {
	return e.GetString("title")
}

func (e *Event) SetTitle(title string) {
	e.Set("title", title)
}

func (e *Event) Start() string {
	return e.GetString("start")
}

func (e *Event) SetStart(start string) {
	e.Set("start", start)
}

func (e *Event) End() string {
	return e.GetString("end")
}

func (e *Event) SetEnd(end string) {
	e.Set("end", end)
}

func (e *Event) AllDay() bool {
	return e.GetBool("allday")
}

func (e *Event) SetAllDay(allDay bool) {
	e.Set("allday", allDay)
}

func (e *Event) Deadline() bool {
	return e.GetBool("deadline")
}

func (e *Event) SetDeadline(deadline bool) {
	e.Set("deadline", deadline)
}

func (e *Event) Announcement() bool {
	return e.GetBool("announcement")
}

func (e *Event) SetAnnouncement(announcement bool) {
	e.Set("announcement", announcement)
}

func (e *Event) Color() string {
	return e.GetString("color")
}

func (e *Event) SetColor(color string) {
	e.Set("color", color)
}

func (e *Event) Description() string {
	return e.GetString("description")
}

func (e *Event) SetDescription(description string) {
	e.Set("description", description)
}

func (e *Event) GoogleEventID() string {
	return e.GetString("google_event_id")
}

func (e *Event) SetGoogleEventID(id string) {
	e.Set("google_event_id", id)
}

func (e *Event) EventLabelID() string {
	return e.GetString("event_label_id")
}

func (e *Event) SetEventLabelID(id string) {
	e.Set("event_label_id", id)
}

func (e *Event) LabelName() string {
	return e.GetString("label_name")
}

func (e *Event) SetLabelName(name string) {
	e.Set("label_name", name)
}

func (e *Event) Recurr() string {
	return e.GetString("recurr")
}

func (e *Event) SetRecurr(rrule string) {
	e.Set("recurr", rrule)
}

func (e *Event) Exdate() any {
	return e.Get("exdate")
}

func (e *Event) SetExdate(exdate any) {
	e.Set("exdate", exdate)
}

func (e *Event) Timezone() string {
	return e.GetString("timezone")
}

func (e *Event) SetTimezone(tz string) {
	e.Set("timezone", tz)
}

func (e *Event) RecurrEventID() string {
	return e.GetString("recurr_event_id")
}

func (e *Event) SetRecurrEventID(id string) {
	e.Set("recurr_event_id", id)
}

func (e *Event) RecurrOgDate() string {
	return e.GetString("recurr_og_date")
}

func (e *Event) SetRecurrOgDate(date string) {
	e.Set("recurr_og_date", date)
}

func (e *Event) Source() string {
	return e.GetString("source")
}

func (e *Event) SetSource(source string) {
	e.Set("source", source)
}
