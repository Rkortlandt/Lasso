package proxies

import (
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// CollectionUsers is the PocketBase collection name for users.
const CollectionUsers = "users"

// Compile-time check ensuring User implements core.RecordProxy
var _ core.RecordProxy = (*User)(nil)

// User wraps a PocketBase Record from the "users" collection.
type User struct {
	core.BaseRecordProxy
}

// NewUser wraps an existing core.Record in a User proxy.
func NewUser(record *core.Record) *User {
	u := &User{}
	u.SetProxyRecord(record)
	return u
}

// NewUserRecord creates a new record from the given collection and wraps it in a User proxy.
func NewUserRecord(col *core.Collection) *User {
	rec := core.NewRecord(col)
	u := &User{}
	u.SetProxyRecord(rec)
	return u
}

// Standard Auth Fields

func (u *User) Email() string {
	return u.GetString("email")
}

func (u *User) SetEmail(email string) {
	u.Set("email", email)
}

func (u *User) EmailVisibility() bool {
	return u.GetBool("emailVisibility")
}

func (u *User) SetEmailVisibility(visible bool) {
	u.Set("emailVisibility", visible)
}

func (u *User) Verified() bool {
	return u.GetBool("verified")
}

func (u *User) SetVerified(verified bool) {
	u.Set("verified", verified)
}

func (u *User) Name() string {
	return u.GetString("name")
}

func (u *User) SetName(name string) {
	u.Set("name", name)
}

func (u *User) Avatar() string {
	return u.GetString("avatar")
}

func (u *User) SetAvatar(avatar string) {
	u.Set("avatar", avatar)
}

// Canvas LMS Auth Fields

func (u *User) CanvasURL() string {
	return u.GetString("canvas_url")
}

func (u *User) SetCanvasURL(url string) {
	u.Set("canvas_url", url)
}

func (u *User) CanvasToken() string {
	return u.GetString("canvas_token")
}

func (u *User) SetCanvasToken(token string) {
	u.Set("canvas_token", token)
}

func (u *User) CanvasConnected() bool {
	return u.GetBool("canvas_connected")
}

func (u *User) SetCanvasConnected(connected bool) {
	u.Set("canvas_connected", connected)
}

func (u *User) CanvasStudentName() string {
	return u.GetString("canvas_student_name")
}

func (u *User) SetCanvasStudentName(name string) {
	u.Set("canvas_student_name", name)
}

// Google Calendar Auth Fields

func (u *User) GoogleEmail() string {
	return u.GetString("google_email")
}

func (u *User) SetGoogleEmail(email string) {
	u.Set("google_email", email)
}

func (u *User) GoogleAccessToken() string {
	return u.GetString("google_access_token")
}

func (u *User) SetGoogleAccessToken(token string) {
	u.Set("google_access_token", token)
}

func (u *User) GoogleRefreshToken() string {
	return u.GetString("google_refresh_token")
}

func (u *User) SetGoogleRefreshToken(token string) {
	u.Set("google_refresh_token", token)
}

func (u *User) GoogleTokenExpiry() string {
	return u.GetString("google_token_expiry")
}

func (u *User) GoogleTokenExpiryTime() time.Time {
	return u.GetDateTime("google_token_expiry").Time()
}

func (u *User) SetGoogleTokenExpiry(expiry any) {
	u.Set("google_token_expiry", expiry)
}

func (u *User) GoogleConnected() bool {
	return u.GetBool("google_connected")
}

func (u *User) SetGoogleConnected(connected bool) {
	u.Set("google_connected", connected)
}
