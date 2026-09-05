# Backend Proxy Refactoring Checklist (`backend/proxies`)

This checklist tracks the refactoring of raw PocketBase `*core.Record` manipulations, untyped strings, and manual dictionary parsing into strongly typed `backend/proxies` models across the backend.

---

## 1. Overview of Proxy Types

All models are defined in [`backend/proxies/`](backend/proxies):
- **[`proxies.User`](backend/proxies/users.go)** (`users` collection): Auth, Google tokens, Canvas tokens, profile.
- **[`proxies.Calendar`](backend/proxies/calendars.go)** (`calendars` collection): Name, nickname, color, source (`todo`, `internal`, `canvas`, `google`), visible, course_id, calendar_id.
- **[`proxies.Task`](backend/proxies/tasks.go)** (`tasks` collection): Name, status (`todo`, `done`), priority (`low`, `med`, `high`), due_date, fake_due_date, grade, source_link.
- **[`proxies.Event`](backend/proxies/events.go)** (`events` collection): Title, start, end, allday, deadline, announcement, color, description, google_event_id, event_label_id, label_name, recurr, exdate, timezone.
- **[`proxies.SyncStatus`](backend/proxies/sync_status.go)** (`sync_status` collection): Status states, timestamps, errors, feedback, and history array.

---

## 2. Status Summary

| File | Status | Notes |
| :--- | :--- | :--- |
| [`backend/canvas.go`](backend/canvas.go) | **Completed** | Fully migrated to `proxies.User`, `proxies.Calendar`, `proxies.Task`, and `proxies.Event` |
| [`backend/sync.go`](backend/sync.go) | **Pending** | Replaces local types and manual JSON history handling with `proxies.SyncStatus` |
| [`backend/google.go`](backend/google.go) | **Pending** | Inbound sync, outbound export, Lasso calendar setup, and token helpers |
| [`backend/main.go`](backend/main.go) | **Pending** | User creation hook, OAuth2 handlers, disconnect routes, event hooks, nickname routes |
| [`backend/item_details.go`](backend/item_details.go) | **Pending** | Reads `canvas_url` and `canvas_token` via `authRecord` |

---

## 3. Detailed Remaining Refactor Locations

### A. [`backend/sync.go`](backend/sync.go)

- [ ] **Remove Redundant Types (Lines 12–28)**
  - Delete local `SyncOperation` and `SyncHistoryLogEntry`.
  - Use `proxies.SyncService`, `proxies.SyncStatusState`, `proxies.SyncResult`, and `proxies.SyncHistoryEntry`.
- [ ] **Refactor `getOrCreateSyncStatus` (Lines 31–55)**
  - Use `proxies.CollectionSyncStatus`.
  - Replace raw `core.NewRecord` and untyped `.Set(...)` with `status := proxies.NewSyncStatusRecord(col)`.
  - Use typed setters: `status.SetUserID(userID)`, `status.SetCanvasStatus(proxies.SyncStatusStateIdle)`, etc.
- [ ] **Refactor `updateSyncStatusRunning` (Lines 58–77)**
  - Wrap record in `status := proxies.NewSyncStatus(rec)`.
  - Use `status.SetCanvasStatus(proxies.SyncStatusStateRunning)`, `status.SetGoogleImportStatus(...)`, `status.SetGoogleExportStatus(...)`.
- [ ] **Refactor `updateSyncStatusFinished` (Lines 80–164)**
  - Replace manual JSON unmarshaling and slice prepending with `status.AppendHistory(...)` or typed `status.History()` / `status.SetHistory()`.
  - Use typed timestamp and feedback setters (`status.SetCanvasSyncedAt(nowISO)`, `status.SetCanvasFeedback(...)`, etc.).

---

### B. [`backend/canvas.go`](backend/canvas.go) (Completed)

- [x] **Auth Token Reading**
  - Uses `user := proxies.NewUser(authRecord); user.CanvasURL()`, `user.CanvasToken()`.
- [x] **User Profile Update in `canvasVerify`**
  - Uses `user.SetCanvasConnected(true)` and `user.SetCanvasStudentName(...)`.
- [x] **Canvas Course Calendar Upsert in `canvasSync`**
  - Uses `proxies.CollectionCalendars`, `proxies.NewCalendar(existingCalendarRecord)`, `proxies.NewCalendarRecord(...)`, and typed methods (`cal.SetSource(proxies.CalendarSourceCanvas)`, etc.).
- [x] **Context/Fallback Calendar Creation in `canvasSync`**
  - Uses `newCal := proxies.NewCalendarRecord(calendarsCollection)` and typed setters.
- [x] **Announcement Event Upsert in `canvasSync`**
  - Uses `proxies.NewEvent(existingAnn)` / `proxies.NewEventRecord(eventsCollection)` with typed setters (`SetAnnouncement`, `SetDeadline`, `SetStart`, etc.).
  - Uses `proxies.CollectionTasks` and `proxies.CollectionEvents`.

---

### C. [`backend/google.go`](backend/google.go)

- [ ] **Google Token & Auth Helpers (Lines 245–260, 340–350, 435–445)**
  - Replace `authRecord.GetString("google_email")`, `authRecord.GetString("google_access_token")`, `authRecord.GetString("google_refresh_token")` with `user := proxies.NewUser(authRecord)`.
  - Update `authRecord.Set("google_access_token", ...)` with `user.SetGoogleAccessToken(...)`.
- [ ] **Dedicated "Lasso" Google Calendar Setup (`ensureLassoGoogleCalendar`, Lines 505–595)**
  - Replace raw `existingRecord.GetString("calendar_id")` and `existingRecord.Set("calendar_id", ...)` with `cal := proxies.NewCalendar(existingRecord)`.
  - Replace `newCal := core.NewRecord(calendarsCollection)` with `newCal := proxies.NewCalendarRecord(calendarsCollection)`.
  - Use `newCal.SetSource(proxies.CalendarSourceGoogle)`, `newCal.SetName("Lasso")`, `newCal.SetColor("#515151")`, `newCal.SetCalendarID(...)`.
- [ ] **Outbound Coursework Export (`syncLassoToGoogle`, Lines 700–1100)**
  - **Calendars:** Wrap course calendars with `proxies.NewCalendar(c)` and read `cal.CourseID()`, `cal.Name()`, `cal.Nickname()`.
  - **Tasks:** Wrap tasks with `proxies.NewTask(task)` and read `t.DueDate()`, `t.FakeDueDate()`, `t.Name()`, `t.Status() == proxies.TaskStatusDone`, `t.Priority()`.
  - **Events / Work Blocks:** Wrap events with `proxies.NewEvent(evt)` and read `e.GoogleEventID()`, `e.CalendarID()`, `e.TaskID()`, `e.Start()`, `e.End()`, `e.Title()`.
- [ ] **Inbound Google Calendar Ingestion (`runInboundGoogleSync`, Lines 1335–1615)**
  - **Calendar Upsert (Lines 1405–1449):** Use `proxies.NewCalendar(existingCal)` / `proxies.NewCalendarRecord(calendarsCol)` with `.SetSource(proxies.CalendarSourceGoogle)`, `.Color()`, `.SetColor()`, `.SetName()`.
  - **Event Upsert (Lines 1541–1574):** Use `proxies.NewEvent(existingEvt)` / `proxies.NewEventRecord(eventsCol)` with `.SetCalendarID()`, `.SetGoogleEventID()`, `.SetTitle()`, `.SetStart()`, `.SetEnd()`, `.SetAllDay()`, `.SetColor()`, `.SetEventLabelID()`, `.SetLabelName()`, `.SetDescription()`.
  - **Pruning Deleted Events (Lines 1577–1605):** Wrap records with `proxies.NewEvent(rec)` and read `.GoogleEventID()`, `.Start()`.

---

### D. [`backend/main.go`](backend/main.go)

- [ ] **Token Refresh Handler (`refreshGoogleToken`, Lines 120–175)**
  - Wrap `authRecord` in `proxies.NewUser(authRecord)`.
  - Replace `authRecord.GetString("google_refresh_token")` and `authRecord.Set(...)` with `user.GoogleRefreshToken()`, `user.SetGoogleAccessToken(...)`, `user.SetGoogleConnected(true)`.
- [ ] **OAuth2 Login & Token Linking Hook (Lines 180–240)**
  - Wrap `e.Record` in `user := proxies.NewUser(e.Record)`.
  - Use typed setters: `user.SetGoogleAccessToken(...)`, `user.SetGoogleRefreshToken(...)`, `user.SetGoogleTokenExpiry(...)`, `user.SetGoogleConnected(true)`, `user.SetGoogleEmail(...)`.
- [ ] **Event Exclusion Hooks (`app.OnRecordCreate("events")` & `OnRecordUpdate("events")`, Lines 269–285)**
  - Wrap `e.Record` in `evt := proxies.NewEvent(e.Record)`.
  - Use `evt.Announcement()`, `evt.Deadline()`, `evt.SetDeadline(false)`.
- [ ] **Server Startup Conflicting Events & Users Cleanup (`app.OnServe()`, Lines 360–390)**
  - Wrap events in `proxies.NewEvent(r)` and call `evt.SetDeadline(false)`.
  - Use `proxies.CollectionEvents` and `proxies.CollectionUsers`.
- [ ] **Default "To Do" Calendar Creation (`ensureTodoCalendar` & `OnRecordCreate("users")`, Lines 248–265, 680–705)**
  - Use `proxies.CollectionCalendars`.
  - Replace raw `core.NewRecord` with `newCal := proxies.NewCalendarRecord(col)`.
  - Use `newCal.SetUserID(userID)`, `newCal.SetName("To Do")`, `newCal.SetSource(proxies.CalendarSourceTodo)`, `newCal.SetColor("#10b981")`, `newCal.SetVisible(true)`.
- [ ] **Calendar Nickname Endpoints (Lines 460–550, 625–650)**
  - Wrap queried calendars with `proxies.NewCalendar(cal)`.
  - Use `cal.Nickname()`, `cal.SetNickname(...)`, `cal.CourseID()`, `cal.CalendarID()`, `cal.Name()`.
- [ ] **Disconnect Routes (`canvasDisconnect` & `googleDisconnect`, Lines 565–575, 660–670)**
  - Wrap `authRecord` in `user := proxies.NewUser(authRecord)`.
  - Use `user.SetCanvasConnected(false)`, `user.SetCanvasToken("")`, `user.SetGoogleConnected(false)`, `user.SetGoogleAccessToken("")`, etc.

---

### E. [`backend/item_details.go`](backend/item_details.go)

- [ ] **Canvas Credentials Lookup (Lines 28–34)**
  - Wrap `authRecord` in `user := proxies.NewUser(authRecord)`.
  - Replace `authRecord.GetString("canvas_url")` and `authRecord.GetString("canvas_token")` with `user.CanvasURL()` and `user.CanvasToken()`.
