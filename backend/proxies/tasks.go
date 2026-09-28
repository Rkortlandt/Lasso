package proxies

import (
	"github.com/pocketbase/pocketbase/core"
)

// CollectionTasks is the PocketBase collection name for tasks.
const CollectionTasks = "tasks"

// TaskStatus represents valid values for task status.
type TaskStatus string

const (
	TaskStatusTodo TaskStatus = "todo"
	TaskStatusDone TaskStatus = "done"
)

// TaskPriority represents valid values for task priority.
type TaskPriority string

const (
	TaskPriorityLow  TaskPriority = "low"
	TaskPriorityMed  TaskPriority = "med"
	TaskPriorityHigh TaskPriority = "high"
)

// TaskSource represents valid values for task source.
type TaskSource string

const (
	TaskSourceTodo   TaskSource = "todo"
	TaskSourceCanvas TaskSource = "canvas"
	TaskSourceGoogle TaskSource = "google"
)

// Compile-time check ensuring Task implements core.RecordProxy
var _ core.RecordProxy = (*Task)(nil)

// Task wraps a PocketBase Record from the "tasks" collection.
type Task struct {
	core.BaseRecordProxy
}

// NewTask wraps an existing core.Record in a Task proxy.
func NewTask(record *core.Record) *Task {
	t := &Task{}
	t.SetProxyRecord(record)
	return t
}

// NewTaskRecord creates a new record from the given collection and wraps it in a Task proxy.
func NewTaskRecord(col *core.Collection) *Task {
	rec := core.NewRecord(col)
	t := &Task{}
	t.SetProxyRecord(rec)
	return t
}

// Relation: User

func (t *Task) UserID() string {
	return t.GetString("user")
}

func (t *Task) SetUserID(userID string) {
	t.Set("user", userID)
}

// User returns the expanded User proxy if loaded via PocketBase expand, or nil.
func (t *Task) User() *User {
	if rec := t.ExpandedOne("user"); rec != nil {
		return NewUser(rec)
	}
	return nil
}

// SetUser assigns the foreign key from a User proxy.
func (t *Task) SetUser(user *User) {
	if user != nil {
		t.SetUserID(user.Id)
	} else {
		t.SetUserID("")
	}
}

// Relation: Calendar

func (t *Task) CalendarID() string {
	id := t.GetString("calendar")
	if id == "" {
		id = t.GetString("calendar_id")
	}
	return id
}

func (t *Task) SetCalendarID(calendarID string) {
	t.Set("calendar", calendarID)
	if t.Record != nil && t.Collection() != nil && t.Collection().Fields.GetByName("calendar_id") != nil {
		t.Set("calendar_id", calendarID)
	}
}

// Calendar returns the expanded Calendar proxy if loaded via PocketBase expand, or nil.
func (t *Task) Calendar() *Calendar {
	if rec := t.ExpandedOne("calendar"); rec != nil {
		return NewCalendar(rec)
	}
	return nil
}

// SetCalendar assigns the foreign key from a Calendar proxy.
func (t *Task) SetCalendar(cal *Calendar) {
	if cal != nil {
		t.SetCalendarID(cal.Id)
	} else {
		t.SetCalendarID("")
	}
}

// Field Getters and Setters

func (t *Task) Name() string {
	return t.GetString("name")
}

func (t *Task) SetName(name string) {
	t.Set("name", name)
}

func (t *Task) Status() TaskStatus {
	return TaskStatus(t.GetString("status"))
}

func (t *Task) SetStatus(status TaskStatus) {
	t.Set("status", string(status))
}

func (t *Task) Priority() TaskPriority {
	return TaskPriority(t.GetString("priority"))
}

func (t *Task) SetPriority(priority TaskPriority) {
	t.Set("priority", string(priority))
}

func (t *Task) DueDate() string {
	return t.GetString("due_date")
}

func (t *Task) SetDueDate(dueDate string) {
	t.Set("due_date", dueDate)
}

func (t *Task) FakeDueDate() string {
	return t.GetString("fake_due_date")
}

func (t *Task) SetFakeDueDate(fakeDueDate string) {
	t.Set("fake_due_date", fakeDueDate)
}

func (t *Task) Grade() string {
	return t.GetString("grade")
}

func (t *Task) SetGrade(grade string) {
	t.Set("grade", grade)
}

func (t *Task) SourceLink() string {
	return t.GetString("source_link")
}

func (t *Task) SetSourceLink(link string) {
	t.Set("source_link", link)
}

func (t *Task) Source() TaskSource {
	return TaskSource(t.GetString("source"))
}

func (t *Task) SetSource(source TaskSource) {
	t.Set("source", string(source))
}

