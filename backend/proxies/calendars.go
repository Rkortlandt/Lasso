package proxies

import (
	"github.com/pocketbase/pocketbase/core"
)

// CollectionCalendars is the PocketBase collection name for calendars.
const CollectionCalendars = "calendars"

// CalendarSource represents valid values for calendar source.
type CalendarSource string

const (
	CalendarSourceTodo     CalendarSource = "todo"
	CalendarSourceInternal CalendarSource = "internal"
	CalendarSourceCanvas   CalendarSource = "canvas"
	CalendarSourceGoogle   CalendarSource = "google"
)

// Compile-time check ensuring Calendar implements core.RecordProxy
var _ core.RecordProxy = (*Calendar)(nil)

// Calendar wraps a PocketBase Record from the "calendars" collection.
type Calendar struct {
	core.BaseRecordProxy
}

// NewCalendar wraps an existing core.Record in a Calendar proxy.
func NewCalendar(record *core.Record) *Calendar {
	c := &Calendar{}
	c.SetProxyRecord(record)
	return c
}

// NewCalendarRecord creates a new record from the given collection and wraps it in a Calendar proxy.
func NewCalendarRecord(col *core.Collection) *Calendar {
	rec := core.NewRecord(col)
	c := &Calendar{}
	c.SetProxyRecord(rec)
	return c
}

// Relation: User

func (c *Calendar) UserID() string {
	return c.GetString("user")
}

func (c *Calendar) SetUserID(userID string) {
	c.Set("user", userID)
}

// User returns the expanded User proxy if loaded via PocketBase expand, or nil.
func (c *Calendar) User() *User {
	if rec := c.ExpandedOne("user"); rec != nil {
		return NewUser(rec)
	}
	return nil
}

// SetUser assigns the foreign key from a User proxy.
func (c *Calendar) SetUser(user *User) {
	if user != nil {
		c.SetUserID(user.Id)
	} else {
		c.SetUserID("")
	}
}

// Relation: Label

func (c *Calendar) LabelID() string {
	return c.GetString("label")
}

func (c *Calendar) SetLabelID(labelID string) {
	c.Set("label", labelID)
}

// Label returns the expanded Label proxy if loaded via PocketBase expand, or nil.
func (c *Calendar) Label() *Label {
	if rec := c.ExpandedOne("label"); rec != nil {
		return NewLabel(rec)
	}
	return nil
}

// SetLabel assigns the foreign key from a Label proxy.
func (c *Calendar) SetLabel(label *Label) {
	if label != nil {
		c.SetLabelID(label.Id)
	} else {
		c.SetLabelID("")
	}
}

// Field Getters and Setters

func (c *Calendar) Name() string {
	return c.GetString("name")
}

func (c *Calendar) SetName(name string) {
	c.Set("name", name)
}

func (c *Calendar) Color() string {
	return c.GetString("color")
}

func (c *Calendar) SetColor(color string) {
	c.Set("color", color)
}

func (c *Calendar) Source() CalendarSource {
	return CalendarSource(c.GetString("source"))
}

func (c *Calendar) SetSource(source CalendarSource) {
	c.Set("source", string(source))
}

func (c *Calendar) Visible() bool {
	return c.GetBool("visible")
}

func (c *Calendar) SetVisible(visible bool) {
	c.Set("visible", visible)
}

func (c *Calendar) Nickname() string {
	return c.GetString("nickname")
}

func (c *Calendar) SetNickname(nickname string) {
	c.Set("nickname", nickname)
}

func (c *Calendar) CourseID() string {
	return c.GetString("course_id")
}

func (c *Calendar) SetCourseID(courseID string) {
	c.Set("course_id", courseID)
}

func (c *Calendar) CalendarID() string {
	return c.GetString("calendar_id")
}

func (c *Calendar) SetCalendarID(calendarID string) {
	c.Set("calendar_id", calendarID)
}

func (c *Calendar) StartDate() string {
	return c.GetString("start_date")
}

func (c *Calendar) SetStartDate(startDate string) {
	c.Set("start_date", startDate)
}

func (c *Calendar) EndDate() string {
	return c.GetString("end_date")
}

func (c *Calendar) SetEndDate(endDate string) {
	c.Set("end_date", endDate)
}

func (c *Calendar) GoogleLabelID() string {
	return c.GetString("google_label_id")
}

func (c *Calendar) SetGoogleLabelID(googleLabelID string) {
	c.Set("google_label_id", googleLabelID)
}
