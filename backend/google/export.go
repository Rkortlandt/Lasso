package google

import (
	"backend/proxies"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type GoogleDateOrDateTime struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type GoogleEventExtendedProperties struct {
	Private map[string]string `json:"private,omitempty"`
	Shared  map[string]string `json:"shared,omitempty"`
}

type GoogleEventPayload struct {
	ID                 string                         `json:"id,omitempty"`
	Summary            string                         `json:"summary"`
	Description        string                         `json:"description,omitempty"`
	Start              GoogleDateOrDateTime           `json:"start"`
	End                GoogleDateOrDateTime           `json:"end"`
	Recurrence         []string                       `json:"recurrence,omitempty"`
	RecurringEventID   string                         `json:"recurringEventId,omitempty"`
	OriginalStartTime  *GoogleDateOrDateTime          `json:"originalStartTime,omitempty"`
	ColorID            string                         `json:"colorId,omitempty"`
	EventLabelID       string                         `json:"eventLabelId,omitempty"`
	ExtendedProperties *GoogleEventExtendedProperties `json:"extendedProperties,omitempty"`
}

type GetGoogleCalendarsResponse struct {
	Success       bool             `json:"success"`
	Connected     bool             `json:"connected"`
	NeedsReauth   bool             `json:"needsReauth,omitempty"`
	Email         string           `json:"email"`
	CalendarCount int              `json:"calendarCount,omitempty"`
	Calendars     []map[string]any `json:"calendars,omitempty"`
	Colors        map[string]any   `json:"colors,omitempty"`
	Message       string           `json:"message,omitempty"`
}

type GoogleCalendarEntry struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

type GoogleSyncResponse struct {
	Success       bool   `json:"success"`
	CalendarID    string `json:"calendarId"`
	SyncedCount   int    `json:"syncedCount"`
	CreatedCount  int    `json:"createdCount"`
	UpdatedCount  int    `json:"updatedCount"`
	DeletedCount  int    `json:"deletedCount,omitempty"`
	CoursesTagged int    `json:"coursesTagged"`
	Message       string `json:"message"`
}

type GoogleListResponse struct {
	Items []map[string]any `json:"items"`
}

var syncStatusMu sync.Mutex

type SyncOperation string

const (
	SyncOpGoogleExport SyncOperation = "google_export"
	SyncOpGoogleImport SyncOperation = "google_import"
)

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

func updateSyncStatusRunning(app core.App, userID string, op SyncOperation) error {
	syncStatusMu.Lock()
	defer syncStatusMu.Unlock()

	rec, err := getOrCreateSyncStatus(app, userID)
	if err != nil {
		return err
	}

	switch op {
	case SyncOpGoogleExport:
		rec.SetGoogleExportStatus(proxies.SyncStatusStateRunning)
		rec.SetGoogleExportError("")
	case SyncOpGoogleImport:
		rec.SetGoogleImportStatus(proxies.SyncStatusStateRunning)
		rec.SetGoogleImportError("")
	}

	return app.Save(rec)
}

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
	case SyncOpGoogleExport:
		rec.SetGoogleExportStatus(proxies.SyncStatusStateIdle)
		if status == "success" {
			rec.SetGoogleExportSyncedAt(nowISO)
			rec.SetGoogleExportFeedback(feedback)
			rec.SetGoogleExportError("")
		} else {
			rec.SetGoogleExportError(errMsg)
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
	}

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

var googleCalendar24Palette = map[string]string{
	"1":  "#ac725e",
	"2":  "#d06b64",
	"3":  "#f83a22",
	"4":  "#fa573c",
	"5":  "#ff7537",
	"6":  "#ffad46",
	"7":  "#42d692",
	"8":  "#16a765",
	"9":  "#7bd148",
	"10": "#b3dc6c",
	"11": "#fbe983",
	"12": "#fad165",
	"13": "#92e1c0",
	"14": "#9fe1e7",
	"15": "#9fc6e7",
	"16": "#4986e7",
	"17": "#9a9cff",
	"18": "#b99aff",
	"19": "#c2c2c2",
	"20": "#cabdbf",
	"21": "#cca6ac",
	"22": "#f691b2",
	"23": "#cd74e6",
	"24": "#a47ae2",
}

/*
Makes a google api request for the calendarlist
Then it checks for errs from the response func, checks for bad http status codes, formats json as a GoogleListResponse
AI_GEN=FALSE
HUMAN_REV=TRUE
*/
func requestCalendarList(app core.App, authUser *proxies.User, e *core.RequestEvent) (data GoogleListResponse, err error) {
	apiResponse, _, err := makeGoogleAPIRequestProx(app, authUser, "GET", "https://www.googleapis.com/calendar/v3/users/me/calendarList", nil)

	if err != nil {
		return GoogleListResponse{}, e.JSON(http.StatusOK, GetGoogleCalendarsResponse{
			Success:     false,
			Connected:   false,
			NeedsReauth: true,
			Email:       authUser.Email(),
			Message:     "Google Calendar credentials not found or expired. Please connect Google Calendar.",
		})
	}
	defer apiResponse.Body.Close()

	if apiResponse.StatusCode != http.StatusOK {
		return GoogleListResponse{}, e.JSON(http.StatusOK, GetGoogleCalendarsResponse{
			Success:     false,
			Connected:   false,
			NeedsReauth: true,
			Email:       authUser.Email(),
			Message:     fmt.Sprintf("Google rejected credentials (HTTP %d). Please reconnect Google Calendar.", apiResponse.StatusCode),
		})
	}

	var formatedResp GoogleListResponse

	respBytes, _ := io.ReadAll(apiResponse.Body)
	if err := json.Unmarshal(respBytes, &formatedResp); err != nil {
		return GoogleListResponse{}, e.BadRequestError("Failed to parse Google Calendar response", err)
	}

	return formatedResp, nil
}

// =============================================================================
// Ensure Dedicated "Lasso" Calendar
// =============================================================================

// ensureLassoGoogleCalendar locates or creates a secondary calendar named "Lasso"
// in the user's Google Calendar account and records it in PocketBase.
func ensureLassoGoogleCalendar(app core.App, authRecord *core.Record) (string, error) {
	calendarsCollection, err := app.FindCollectionByNameOrId("calendars")
	if err != nil {
		return "", fmt.Errorf("calendars collection not found: %w", err)
	}

	// 1. Check if user already has a 'Lasso' calendar record in PB with calendar_id
	existingRecord, _ := app.FindFirstRecordByFilter(
		"calendars",
		"user = {:user} && name = 'Lasso' && calendar_id != ''",
		map[string]any{"user": authRecord.Id},
	)

	// 2. Query Google Calendar list to check if "Lasso" calendar exists in Google
	resp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", "https://www.googleapis.com/calendar/v3/users/me/calendarList?minAccessRole=writer", nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch google calendar list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("google calendarList failed (status %d): %s", resp.StatusCode, string(body))
	}

	var calListResp struct {
		Items []GoogleCalendarEntry `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&calListResp); err != nil {
		return "", fmt.Errorf("failed to decode calendar list: %w", err)
	}

	foundGoogleCalID := ""
	for _, c := range calListResp.Items {
		if strings.EqualFold(strings.TrimSpace(c.Summary), "Lasso") {
			foundGoogleCalID = c.ID
			break
		}
	}

	// 3. If "Lasso" calendar not found in Google, create it!
	if foundGoogleCalID == "" {
		createPayload := map[string]string{
			"summary":     "Lasso",
			"description": "Academic deadlines, schedules, and coursework synced from Lasso",
			"timeZone":    "UTC",
		}
		createBody, _ := json.Marshal(createPayload)

		createResp, _, err := makeGoogleAPIRequest(app, authRecord, "POST", "https://www.googleapis.com/calendar/v3/calendars", createBody)
		if err != nil {
			return "", fmt.Errorf("failed to create Google 'Lasso' calendar: %w", err)
		}
		defer createResp.Body.Close()

		if createResp.StatusCode != http.StatusOK && createResp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(createResp.Body)
			return "", fmt.Errorf("google create calendar failed (%d): %s", createResp.StatusCode, string(body))
		}

		var createdCal GoogleCalendarEntry
		if err := json.NewDecoder(createResp.Body).Decode(&createdCal); err != nil {
			return "", fmt.Errorf("failed to parse created calendar response: %w", err)
		}
		foundGoogleCalID = createdCal.ID
		log.Printf("Successfully created new 'Lasso' secondary calendar in Google (ID: %s) for user %s", foundGoogleCalID, authRecord.Id)
	}

	// 4. Save/Update PB record
	if existingRecord != nil {
		cal := proxies.NewCalendar(existingRecord)
		if cal.CalendarID() != foundGoogleCalID {
			cal.SetCalendarID(foundGoogleCalID)
			cal.SetSource(proxies.CalendarSourceGoogle)
			_ = app.Save(cal)
		}
	} else {
		newCal := proxies.NewCalendarRecord(calendarsCollection)
		newCal.SetUserID(authRecord.Id)
		newCal.SetName("Lasso")
		newCal.SetColor("#515151")
		newCal.SetSource(proxies.CalendarSourceGoogle)
		newCal.SetVisible(true)
		newCal.SetCalendarID(foundGoogleCalID)
		if err := app.Save(newCal); err != nil {
			log.Printf("Warning: failed to save Lasso calendar record in PB: %v", err)
		}
	}

	return foundGoogleCalID, nil
}

// handlePurgeLassoCalendar deletes the existing "Lasso" secondary calendar(s) from Google Calendar
// and recreates a fresh, completely empty one.
func handlePurgeLassoCalendar(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		// 1. Query Google Calendar list to find existing "Lasso" calendar(s)
		resp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", "https://www.googleapis.com/calendar/v3/users/me/calendarList?minAccessRole=writer", nil)
		if err != nil {
			return e.BadRequestError("Failed to fetch google calendar list: "+err.Error(), err)
		}
		defer resp.Body.Close()

		var calListResp struct {
			Items []GoogleCalendarEntry `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&calListResp); err == nil {
			for _, c := range calListResp.Items {
				if strings.EqualFold(strings.TrimSpace(c.Summary), "Lasso") {
					// Delete secondary calendar from Google Calendar
					delURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s", url.PathEscape(c.ID))
					delResp, _, errDel := makeGoogleAPIRequest(app, authRecord, "DELETE", delURL, nil)
					if errDel == nil {
						delResp.Body.Close()
						log.Printf("Successfully deleted Google 'Lasso' calendar %s", c.ID)
					}
				}
			}
		}

		// 2. Remove any existing Lasso calendar records from PocketBase
		pbRecords, _ := app.FindRecordsByFilter(
			"calendars",
			"user = {:user} && name = 'Lasso'",
			"",
			50,
			0,
			map[string]any{"user": authRecord.Id},
		)
		for _, rec := range pbRecords {
			_ = app.Delete(rec)
		}

		// 3. Re-create a fresh, clean Lasso calendar
		newCalID, err := ensureLassoGoogleCalendar(app, authRecord)
		if err != nil {
			return e.BadRequestError("Failed to recreate clean Lasso calendar: "+err.Error(), err)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"success":    true,
			"calendarId": newCalID,
			"message":    "Successfully purged and recreated a fresh 'Lasso' calendar.",
		})
	}
}

// =============================================================================
// Sync Lasso Events & Tasks to Dedicated "Lasso" Google Calendar
// =============================================================================

// handleSyncLassoToGoogle pushes user coursework deadlines and events to the "Lasso" Google Calendar,
// tagging each event by course with title prefix [Course], description metadata, and color.
func handleSyncLassoToGoogle(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		resp, err := runOutboundGoogleSync(app, authRecord)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}

		return e.JSON(http.StatusOK, resp)
	}
}

// runOutboundGoogleSync pushes user coursework deadlines and events to the "Lasso" Google Calendar.
func runOutboundGoogleSync(app core.App, authRecord *core.Record) (*GoogleSyncResponse, error) {
	if authRecord == nil {
		return nil, fmt.Errorf("authentication required")
	}

	startTime := time.Now()

	syncStatus, err := getOrCreateSyncStatus(app, authRecord.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync status: %w", err)
	}

	if !syncStatus.GoogleExportEnabled() {
		msg := "Google Calendar export is disabled in user settings."
		_ = updateSyncStatusFinished(
			app,
			authRecord.Id,
			SyncOpGoogleExport,
			"success",
			msg,
			"",
			time.Since(startTime).Milliseconds(),
		)
		return &GoogleSyncResponse{
			Success: true,
			Message: msg,
		}, nil
	}

	_ = updateSyncStatusRunning(app, authRecord.Id, SyncOpGoogleExport)

	// Target calendar resolution: prefer active record in 'calendars', then cached syncStatus, then ensure
	lassoCalID := ""
	if calRec, _ := app.FindFirstRecordByFilter(proxies.CollectionCalendars, "user = {:user} && name = 'Lasso' && calendar_id != ''", map[string]any{"user": authRecord.Id}); calRec != nil {
		cal := proxies.NewCalendar(calRec)
		lassoCalID = cal.CalendarID()
	}
	if lassoCalID == "" {
		lassoCalID = syncStatus.GoogleExportCalendarID()
	}
	if lassoCalID == "" {
		var errEnsure error
		lassoCalID, errEnsure = ensureLassoGoogleCalendar(app, authRecord)
		if errEnsure != nil {
			_ = updateSyncStatusFinished(app, authRecord.Id, SyncOpGoogleExport, "error", "", errEnsure.Error(), time.Since(startTime).Milliseconds())
			return nil, fmt.Errorf("failed to setup Lasso Google calendar: %w", errEnsure)
		}
	}
	if syncStatus.GoogleExportCalendarID() != lassoCalID {
		syncStatus.SetGoogleExportCalendarID(lassoCalID)
		_ = app.Save(syncStatus)
	}

	// 1. Fetch user coursework calendars
	calRecords, err := app.FindRecordsByFilter(
		proxies.CollectionCalendars,
		"user = {:user}",
		"name",
		200,
		0,
		map[string]any{"user": authRecord.Id},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendars: %w", err)
	}

	var courseCalendars []*proxies.Calendar
	calByID := make(map[string]*proxies.Calendar)
	for _, c := range calRecords {
		cal := proxies.NewCalendar(c)
		courseCalendars = append(courseCalendars, cal)
		calByID[cal.Id] = cal
		if cid := cal.CourseID(); cid != "" {
			calByID[cid] = cal
		}
	}

	// 2. Fetch user labels
	labelRecords, _ := app.FindRecordsByFilter(
		proxies.CollectionLabels,
		"user = {:user}",
		"name",
		200,
		0,
		map[string]any{"user": authRecord.Id},
	)
	var userLabels []*proxies.Label
	for _, l := range labelRecords {
		userLabels = append(userLabels, proxies.NewLabel(l))
	}

	// Phase 1: Flush pending deletions concurrently with Phase 2 (Decoupled Tag Check)
	var deletedCount int
	var tagSyncErr error
	var wg sync.WaitGroup

	// Worker 1: Flush pending_delete tombstones
	wg.Add(1)
	go func() {
		defer wg.Done()
		pendingRecords, errPending := app.FindRecordsByFilter(
			proxies.CollectionSyncRegistry,
			"user = {:user} && google_calendar_id = {:cal} && status = 'pending_delete'",
			"",
			500,
			0,
			map[string]any{"user": authRecord.Id, "cal": lassoCalID},
		)
		if errPending != nil || len(pendingRecords) == 0 {
			return
		}

		var delRecordsToSave []*core.Record
		for _, rawReg := range pendingRecords {
			reg := proxies.NewSyncRegistry(rawReg)
			gEventID := reg.GoogleEventID()
			if gEventID == "" {
				reg.SetStatus(proxies.SyncRegistryStatusDeleted)
				delRecordsToSave = append(delRecordsToSave, reg.Record)
				continue
			}

			delURL := fmt.Sprintf(
				"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s",
				url.PathEscape(lassoCalID),
				url.PathEscape(gEventID),
			)
			delResp, _, errDel := makeGoogleAPIRequest(app, authRecord, "DELETE", delURL, nil)
			if errDel == nil {
				if delResp.StatusCode == http.StatusOK || delResp.StatusCode == http.StatusNoContent || delResp.StatusCode == http.StatusNotFound || delResp.StatusCode == http.StatusGone {
					reg.SetStatus(proxies.SyncRegistryStatusDeleted)
					delRecordsToSave = append(delRecordsToSave, reg.Record)
					deletedCount++
				}
				io.Copy(io.Discard, delResp.Body)
				delResp.Body.Close()
			}
		}
		if len(delRecordsToSave) > 0 {
			_ = app.RunInTransaction(func(txApp core.App) error {
				for _, rec := range delRecordsToSave {
					_ = txApp.Save(rec)
				}
				return nil
			})
		}
	}()

	// Worker 2: Decoupled Tag Check (0 Google API calls if clean, 1 PATCH if dirty)
	wg.Add(1)
	go func() {
		defer wg.Done()
		tagSyncErr = reconcileCalendarTagsIfDirty(app, authRecord, lassoCalID, syncStatus, courseCalendars, userLabels)
	}()

	wg.Wait()

	if tagSyncErr != nil {
		log.Printf("Warning: reconcileCalendarTagsIfDirty returned: %v", tagSyncErr)
	}

	// Phase 3: In-Memory Delta Filter & Targeted Dispatch
	activeRegRecords, err := app.FindRecordsByFilter(
		proxies.CollectionSyncRegistry,
		"user = {:user} && google_calendar_id = {:cal} && status = 'active'",
		"",
		0,
		0,
		map[string]any{"user": authRecord.Id, "cal": lassoCalID},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query sync registry: %w", err)
	}

	regMap := make(map[string]*proxies.SyncRegistry) // key = entityType + ":" + entityID
	for _, rawReg := range activeRegRecords {
		reg := proxies.NewSyncRegistry(rawReg)
		regMap[string(reg.EntityType())+":"+reg.EntityID()] = reg
	}

	// Fetch tasks
	taskRecords, err := app.FindRecordsByFilter(
		proxies.CollectionTasks,
		"user = {:user}",
		"due_date",
		500,
		0,
		map[string]any{"user": authRecord.Id},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tasks: %w", err)
	}

	taskByID := make(map[string]*proxies.Task)
	var validTasks []*proxies.Task
	for _, rawTask := range taskRecords {
		t := proxies.NewTask(rawTask)
		taskByID[t.Id] = t
		if t.DueDate() != "" || t.FakeDueDate() != "" {
			validTasks = append(validTasks, t)
		}
	}

	// Fetch coursework events / work blocks (excluding Google-origin events)
	allEvents, errEvents := app.FindRecordsByFilter(
		proxies.CollectionEvents,
		"",
		"start",
		0,
		0,
		nil,
	)
	if errEvents != nil {
		log.Printf("Warning: failed to query events in export: %v", errEvents)
	}

	var validEvents []*proxies.Event
	for _, rawEvt := range allEvents {
		evt := proxies.NewEvent(rawEvt)
		if evt.GoogleEventID() != "" {
			continue
		}
		if evt.Deadline() {
			continue // Deadlines are synced from tasks
		}

		calID := evt.CalendarID()
		var courseCal *proxies.Calendar
		if calID != "" {
			courseCal = calByID[calID]
		}
		if (courseCal == nil || (courseCal.Source() != proxies.CalendarSourceCanvas && courseCal.LabelID() == "")) && evt.TaskID() != "" {
			if linkedTask := taskByID[evt.TaskID()]; linkedTask != nil {
				if taskCalID := linkedTask.CalendarID(); taskCalID != "" {
					if tCal := calByID[taskCalID]; tCal != nil {
						courseCal = tCal
					}
				}
			}
		}
		if courseCal == nil {
			continue
		}
		if courseCal.Source() == proxies.CalendarSourceGoogle || courseCal.CalendarID() != "" {
			continue
		}
		if courseCal.UserID() != authRecord.Id {
			continue
		}
		if courseCal.Source() != proxies.CalendarSourceCanvas && evt.TaskID() == "" && evt.Recurr() == "" && evt.RecurrEventID() == "" {
			continue
		}
		validEvents = append(validEvents, evt)
	}

	createdCount := 0
	updatedCount := 0
	cleanTasksCount := 0
	dirtyTasksCount := 0
	coursesTaggedSet := make(map[string]bool)

	type dirtyTaskItem struct {
		task           *proxies.Task
		existingReg    *proxies.SyncRegistry
		courseCal      *proxies.Calendar
		courseName     string
		courseNickname string
		canvasCourseID string
		courseTag      string
		rawDue         string
	}

	var dirtyTasks []dirtyTaskItem

	// Delta filter for tasks
	for _, task := range validTasks {
		rawDue := task.DueDate()
		if strings.TrimSpace(rawDue) == "" {
			rawDue = task.FakeDueDate()
		}
		if strings.TrimSpace(rawDue) == "" {
			continue
		}

		calID := task.CalendarID()
		var courseCal *proxies.Calendar
		if calID != "" {
			courseCal = calByID[calID]
		}

		courseName := "Coursework"
		courseNickname := ""
		canvasCourseID := ""
		if courseCal != nil {
			courseName = courseCal.Name()
			courseNickname = courseCal.Nickname()
			canvasCourseID = courseCal.CourseID()
		}

		courseTag := extractCourseTag(courseName, courseNickname)
		coursesTaggedSet[courseTag] = true

		// Delta evaluation
		regKey := string(proxies.SyncRegistryEntityTypeTask) + ":" + task.Id
		existingReg := regMap[regKey]

		isDirty := false
		if existingReg == nil {
			isDirty = true
		} else {
			syncedAtTime := existingReg.GetDateTime("synced_at").Time()
			if syncedAtTime.IsZero() {
				isDirty = true
			} else {
				taskUpdated := task.GetDateTime("updated").Time()
				if taskUpdated.After(syncedAtTime) {
					isDirty = true
				}
			}
		}

		if !isDirty {
			cleanTasksCount++
			continue // 0 Google API calls!
		}
		dirtyTasksCount++
		dirtyTasks = append(dirtyTasks, dirtyTaskItem{
			task:           task,
			existingReg:    existingReg,
			courseCal:      courseCal,
			courseName:     courseName,
			courseNickname: courseNickname,
			canvasCourseID: canvasCourseID,
			courseTag:      courseTag,
			rawDue:         rawDue,
		})
	}

	// Parallel dispatch for dirty tasks
	if len(dirtyTasks) > 0 {
		regCol, _ := app.FindCollectionByNameOrId(proxies.CollectionSyncRegistry)
		const maxConcurrency = 4
		sem := make(chan struct{}, maxConcurrency)
		var taskWg sync.WaitGroup
		var mu sync.Mutex
		var taskRegsToSave []*core.Record

		for _, dt := range dirtyTasks {
			taskWg.Add(1)
			sem <- struct{}{}
			go func(item dirtyTaskItem) {
				defer func() {
					<-sem
					taskWg.Done()
				}()

				taskTitle := item.task.Name()
				taggedSummary := fmt.Sprintf("[%s] %s", item.courseTag, taskTitle)
				if item.task.Status() == proxies.TaskStatusDone {
					taggedSummary = fmt.Sprintf("[%s] ✓ %s", item.courseTag, taskTitle)
				}

				parsedTime, isDateOnly, errParse := parseDateString(item.rawDue)
				if errParse != nil {
					log.Printf("Skipping task %s due to unparseable date '%s': %v", item.task.Id, item.rawDue, errParse)
					return
				}

				var startObj, endObj GoogleDateOrDateTime
				if isDateOnly {
					dateStr := parsedTime.Format("2006-01-02")
					startObj = GoogleDateOrDateTime{Date: dateStr}
					endObj = GoogleDateOrDateTime{Date: parsedTime.AddDate(0, 0, 1).Format("2006-01-02")}
				} else {
					endObj = GoogleDateOrDateTime{DateTime: parsedTime.Format(time.RFC3339)}
					startObj = GoogleDateOrDateTime{DateTime: parsedTime.Add(-30 * time.Minute).Format(time.RFC3339)}
				}

				statusText := "To Do"
				if item.task.Status() == proxies.TaskStatusDone {
					statusText = "Completed"
				}
				priorityText := strings.ToUpper(string(item.task.Priority()))
				if priorityText == "" {
					priorityText = "NORMAL"
				}

				description := fmt.Sprintf(
					"Course: %s\nStatus: %s\nPriority: %s\nDue: %s\n\nSynced by Lasso",
					item.courseName,
					statusText,
					priorityText,
					item.rawDue,
				)

				eventLabelID := ""
				if item.courseCal != nil {
					eventLabelID = item.courseCal.GoogleLabelID()
				}

				payload := GoogleEventPayload{
					Summary:      taggedSummary,
					Description:  description,
					Start:        startObj,
					End:          endObj,
					EventLabelID: eventLabelID,
					ExtendedProperties: &GoogleEventExtendedProperties{
						Private: map[string]string{
							"lasso_managed":     "true",
							"lasso_task_id":     item.task.Id,
							"lasso_course_id":   item.canvasCourseID,
							"lasso_course_name": item.courseName,
							"lasso_tag":         item.courseTag,
						},
					},
				}
				payloadBytes, _ := json.Marshal(payload)

				labelQuery := ""
				if eventLabelID != "" {
					labelQuery = "?eventLabelVersion=1"
				}

				if item.existingReg != nil && item.existingReg.GoogleEventID() != "" {
					// Update existing event (PATCH)
					patchURL := fmt.Sprintf(
						"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s%s",
						url.PathEscape(lassoCalID),
						url.PathEscape(item.existingReg.GoogleEventID()),
						labelQuery,
					)
					resp, _, errPatch := makeGoogleAPIRequest(app, authRecord, "PATCH", patchURL, payloadBytes)
					if errPatch != nil {
						log.Printf("[GoogleExport] Network error patching task %s: %v", item.task.Id, errPatch)
					} else {
						if resp.StatusCode == http.StatusOK {
							item.existingReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
							mu.Lock()
							taskRegsToSave = append(taskRegsToSave, item.existingReg.Record)
							updatedCount++
							mu.Unlock()
						} else if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
							// Re-create via POST if Google event was deleted remotely
							insertURL := fmt.Sprintf(
								"https://www.googleapis.com/calendar/v3/calendars/%s/events%s",
								url.PathEscape(lassoCalID),
								labelQuery,
							)
							respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
							if errPost != nil {
								log.Printf("[GoogleExport] Network error re-creating task %s: %v", item.task.Id, errPost)
							} else {
								if respPost.StatusCode == http.StatusOK || respPost.StatusCode == http.StatusCreated {
									var createdEvt GoogleEventPayload
									if errDec := json.NewDecoder(respPost.Body).Decode(&createdEvt); errDec == nil {
										item.existingReg.SetGoogleEventID(createdEvt.ID)
										item.existingReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
										mu.Lock()
										taskRegsToSave = append(taskRegsToSave, item.existingReg.Record)
										updatedCount++
										mu.Unlock()
									}
								} else {
									respB, _ := io.ReadAll(respPost.Body)
									log.Printf("[GoogleExport] Error re-creating task %s in Google (HTTP %d): %s", item.task.Id, respPost.StatusCode, string(respB))
								}
								io.Copy(io.Discard, respPost.Body)
								respPost.Body.Close()
							}
						} else {
							respB, _ := io.ReadAll(resp.Body)
							log.Printf("[GoogleExport] Error patching task %s in Google (HTTP %d): %s", item.task.Id, resp.StatusCode, string(respB))
						}
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				} else {
					// Create new event (POST)
					insertURL := fmt.Sprintf(
						"https://www.googleapis.com/calendar/v3/calendars/%s/events%s",
						url.PathEscape(lassoCalID),
						labelQuery,
					)
					respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
					if errPost != nil {
						log.Printf("[GoogleExport] Network error inserting task %s: %v", item.task.Id, errPost)
					} else {
						if respPost.StatusCode == http.StatusOK || respPost.StatusCode == http.StatusCreated {
							var createdEvt GoogleEventPayload
							if errDec := json.NewDecoder(respPost.Body).Decode(&createdEvt); errDec == nil {
								if regCol != nil {
									newReg := proxies.NewSyncRegistryRecord(regCol)
									newReg.SetUserID(authRecord.Id)
									newReg.SetEntityID(item.task.Id)
									newReg.SetEntityType(proxies.SyncRegistryEntityTypeTask)
									newReg.SetGoogleCalendarID(lassoCalID)
									newReg.SetGoogleEventID(createdEvt.ID)
									newReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
									newReg.SetStatus(proxies.SyncRegistryStatusActive)
									mu.Lock()
									taskRegsToSave = append(taskRegsToSave, newReg.Record)
									createdCount++
									mu.Unlock()
								}
							}
						} else {
							respB, _ := io.ReadAll(respPost.Body)
							log.Printf("[GoogleExport] Error creating task %s in Google (HTTP %d): %s", item.task.Id, respPost.StatusCode, string(respB))
						}
						io.Copy(io.Discard, respPost.Body)
						respPost.Body.Close()
					}
				}
			}(dt)
		}
		taskWg.Wait()

		if len(taskRegsToSave) > 0 {
			errBatch := app.RunInTransaction(func(txApp core.App) error {
				for _, rec := range taskRegsToSave {
					if err := txApp.Save(rec); err != nil {
						log.Printf("Error batch saving task sync registry record: %v", err)
					}
				}
				return nil
			})
			if errBatch != nil {
				log.Printf("Error committing task sync registry transaction: %v", errBatch)
			}
		}
	}

	log.Printf("[GoogleExport] Tasks evaluated: %d total (%d clean/skipped, %d dirty/dispatched)", len(validTasks), cleanTasksCount, dirtyTasksCount)

	cleanEvtsCount := 0
	dirtyEvtsCount := 0

	type dirtyEventItem struct {
		evt            *proxies.Event
		existingReg    *proxies.SyncRegistry
		courseCal      *proxies.Calendar
		courseName     string
		courseNickname string
		canvasCourseID string
		courseTag      string
		rawStart       string
	}

	var dirtyEvents []dirtyEventItem

	// Delta filter for events (work sessions / Canvas coursework events)
	for _, evt := range validEvents {
		rawStart := evt.Start()
		if strings.TrimSpace(rawStart) == "" {
			continue
		}

		calID := evt.CalendarID()
		var courseCal *proxies.Calendar
		if calID != "" {
			courseCal = calByID[calID]
		}
		if (courseCal == nil || courseCal.LabelID() == "") && evt.TaskID() != "" {
			if linkedTask := taskByID[evt.TaskID()]; linkedTask != nil {
				if taskCalID := linkedTask.CalendarID(); taskCalID != "" {
					if tCal := calByID[taskCalID]; tCal != nil {
						courseCal = tCal
					}
				}
			}
		}

		courseName := "Coursework"
		courseNickname := ""
		canvasCourseID := ""
		if courseCal != nil {
			courseName = courseCal.Name()
			courseNickname = courseCal.Nickname()
			canvasCourseID = courseCal.CourseID()
		}

		courseTag := extractCourseTag(courseName, courseNickname)
		coursesTaggedSet[courseTag] = true

		// Delta evaluation
		regKey := string(proxies.SyncRegistryEntityTypeEvent) + ":" + evt.Id
		existingReg := regMap[regKey]

		isDirty := false
		if existingReg == nil {
			isDirty = true
		} else {
			syncedAtTime := existingReg.GetDateTime("synced_at").Time()
			if syncedAtTime.IsZero() {
				isDirty = true
			} else {
				evtUpdated := evt.GetDateTime("updated").Time()
				if evtUpdated.After(syncedAtTime) {
					isDirty = true
				}
			}
		}

		if !isDirty {
			cleanEvtsCount++
			continue // 0 Google API calls!
		}
		dirtyEvtsCount++
		dirtyEvents = append(dirtyEvents, dirtyEventItem{
			evt:            evt,
			existingReg:    existingReg,
			courseCal:      courseCal,
			courseName:     courseName,
			courseNickname: courseNickname,
			canvasCourseID: canvasCourseID,
			courseTag:      courseTag,
			rawStart:       rawStart,
		})
	}

	log.Printf("[GoogleExport] Events evaluated: %d total (%d clean/skipped, %d dirty/dispatched)", len(validEvents), cleanEvtsCount, dirtyEvtsCount)

	// Parallel dispatch for dirty events
	if len(dirtyEvents) > 0 {
		regCol, _ := app.FindCollectionByNameOrId(proxies.CollectionSyncRegistry)
		const maxConcurrency = 4
		sem := make(chan struct{}, maxConcurrency)
		var evtWg sync.WaitGroup
		var mu sync.Mutex
		var evtRegsToSave []*core.Record

		for _, de := range dirtyEvents {
			evtWg.Add(1)
			sem <- struct{}{}
			go func(item dirtyEventItem) {
				defer func() {
					<-sem
					evtWg.Done()
				}()

				evtTitle := item.evt.Title()
				if evtTitle == "" {
					evtTitle = "Event"
				}
				taggedSummary := fmt.Sprintf("[%s] %s", item.courseTag, evtTitle)

				startParsed, isStartOnlyDate, errStart := parseDateString(item.rawStart)
				if errStart != nil {
					log.Printf("Skipping event %s due to unparseable start date '%s': %v", item.evt.Id, item.rawStart, errStart)
					return
				}

				var startObj, endObj GoogleDateOrDateTime
				if item.evt.AllDay() || isStartOnlyDate {
					dateStr := startParsed.Format("2006-01-02")
					startObj = GoogleDateOrDateTime{Date: dateStr}
					rawEnd := item.evt.End()
					if rawEnd != "" {
						if endParsed, _, errEnd := parseDateString(rawEnd); errEnd == nil {
							endObj = GoogleDateOrDateTime{Date: endParsed.AddDate(0, 0, 1).Format("2006-01-02")}
						} else {
							endObj = GoogleDateOrDateTime{Date: startParsed.AddDate(0, 0, 1).Format("2006-01-02")}
						}
					} else {
						endObj = GoogleDateOrDateTime{Date: startParsed.AddDate(0, 0, 1).Format("2006-01-02")}
					}
				} else {
					startObj = GoogleDateOrDateTime{DateTime: startParsed.Format(time.RFC3339)}
					rawEnd := item.evt.End()
					if rawEnd != "" {
						if endParsed, _, errEnd := parseDateString(rawEnd); errEnd == nil {
							endObj = GoogleDateOrDateTime{DateTime: endParsed.Format(time.RFC3339)}
						} else {
							endObj = GoogleDateOrDateTime{DateTime: startParsed.Add(60 * time.Minute).Format(time.RFC3339)}
						}
					} else {
						endObj = GoogleDateOrDateTime{DateTime: startParsed.Add(60 * time.Minute).Format(time.RFC3339)}
					}
				}

				eventLabelID := ""
				if item.courseCal != nil {
					eventLabelID = item.courseCal.GoogleLabelID()
				}

				tz := item.evt.Timezone()
				if tz == "" {
					tz = "UTC"
				}
				if !item.evt.AllDay() && !isStartOnlyDate {
					startObj.TimeZone = tz
					endObj.TimeZone = tz
				}

				payload := GoogleEventPayload{
					Summary:      taggedSummary,
					Description:  fmt.Sprintf("Course: %s\n\nSynced by Lasso", item.courseName),
					Start:        startObj,
					End:          endObj,
					EventLabelID: eventLabelID,
					ExtendedProperties: &GoogleEventExtendedProperties{
						Private: map[string]string{
							"lasso_managed":     "true",
							"lasso_event_id":    item.evt.Id,
							"lasso_course_id":   item.canvasCourseID,
							"lasso_course_name": item.courseName,
							"lasso_tag":         item.courseTag,
						},
					},
				}

				if item.evt.Recurr() != "" {
					rruleStr := item.evt.Recurr()
					if !strings.HasPrefix(rruleStr, "RRULE:") && !strings.HasPrefix(rruleStr, "EXRULE:") {
						rruleStr = "RRULE:" + rruleStr
					}
					payload.Recurrence = []string{rruleStr}

					// Append exception dates (EXDATE) if present
					if rawExdates := item.evt.Exdate(); rawExdates != nil {
						var exdateList []string
						switch val := rawExdates.(type) {
						case []string:
							exdateList = val
						case []any:
							for _, v := range val {
								if s, ok := v.(string); ok && s != "" {
									exdateList = append(exdateList, s)
								}
							}
						case string:
							if val != "" {
								var parsed []string
								if json.Unmarshal([]byte(val), &parsed) == nil {
									exdateList = parsed
								} else {
									exdateList = strings.Split(val, ",")
								}
							}
						}

						for _, ex := range exdateList {
							ex = strings.TrimSpace(ex)
							if ex == "" {
								continue
							}
							if parsedEx, isExDateOnly, errEx := parseDateString(ex); errEx == nil {
								if isExDateOnly || item.evt.AllDay() {
									payload.Recurrence = append(payload.Recurrence, fmt.Sprintf("EXDATE;VALUE=DATE:%s", parsedEx.Format("20060102")))
								} else {
									payload.Recurrence = append(payload.Recurrence, fmt.Sprintf("EXDATE:%s", parsedEx.UTC().Format("20060102T150405Z")))
								}
							}
						}
					}
				}

				if item.evt.RecurrEventID() != "" && item.evt.RecurrOgDate() != "" {
					masterRegKey := string(proxies.SyncRegistryEntityTypeEvent) + ":" + item.evt.RecurrEventID()
					if mReg := regMap[masterRegKey]; mReg != nil && mReg.GoogleEventID() != "" {
						payload.RecurringEventID = mReg.GoogleEventID()
						if origT, isOrigDateOnly, errOrig := parseDateString(item.evt.RecurrOgDate()); errOrig == nil {
							if isOrigDateOnly || item.evt.AllDay() {
								payload.OriginalStartTime = &GoogleDateOrDateTime{Date: origT.Format("2006-01-02")}
							} else {
								payload.OriginalStartTime = &GoogleDateOrDateTime{DateTime: origT.Format(time.RFC3339)}
							}
						}
					}
				}

				payloadBytes, _ := json.Marshal(payload)

				labelQuery := ""
				if eventLabelID != "" {
					labelQuery = "?eventLabelVersion=1"
				}

				if item.existingReg != nil && item.existingReg.GoogleEventID() != "" {
					patchURL := fmt.Sprintf(
						"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s%s",
						url.PathEscape(lassoCalID),
						url.PathEscape(item.existingReg.GoogleEventID()),
						labelQuery,
					)
					resp, _, errPatch := makeGoogleAPIRequest(app, authRecord, "PATCH", patchURL, payloadBytes)
					if errPatch != nil {
						log.Printf("[GoogleExport] Network error patching event %s: %v", item.evt.Id, errPatch)
					} else {
						if resp.StatusCode == http.StatusOK {
							item.existingReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
							mu.Lock()
							evtRegsToSave = append(evtRegsToSave, item.existingReg.Record)
							updatedCount++
							mu.Unlock()
						} else if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
							// Re-create via POST if Google event was deleted remotely
							insertURL := fmt.Sprintf(
								"https://www.googleapis.com/calendar/v3/calendars/%s/events%s",
								url.PathEscape(lassoCalID),
								labelQuery,
							)
							respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
							if errPost != nil {
								log.Printf("[GoogleExport] Network error re-creating event %s: %v", item.evt.Id, errPost)
							} else {
								if respPost.StatusCode == http.StatusOK || respPost.StatusCode == http.StatusCreated {
									var createdEvt GoogleEventPayload
									if errDec := json.NewDecoder(respPost.Body).Decode(&createdEvt); errDec == nil {
										item.existingReg.SetGoogleEventID(createdEvt.ID)
										item.existingReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
										mu.Lock()
										evtRegsToSave = append(evtRegsToSave, item.existingReg.Record)
										updatedCount++
										mu.Unlock()
									}
								} else {
									respB, _ := io.ReadAll(respPost.Body)
									log.Printf("[GoogleExport] Error re-creating event %s in Google (HTTP %d): %s", item.evt.Id, respPost.StatusCode, string(respB))
								}
								io.Copy(io.Discard, respPost.Body)
								respPost.Body.Close()
							}
						} else {
							respB, _ := io.ReadAll(resp.Body)
							log.Printf("[GoogleExport] Error patching event %s in Google (HTTP %d): %s", item.evt.Id, resp.StatusCode, string(respB))
						}
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				} else {
					insertURL := fmt.Sprintf(
						"https://www.googleapis.com/calendar/v3/calendars/%s/events%s",
						url.PathEscape(lassoCalID),
						labelQuery,
					)
					respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
					if errPost != nil {
						log.Printf("[GoogleExport] Network error inserting event %s: %v", item.evt.Id, errPost)
					} else {
						if respPost.StatusCode == http.StatusOK || respPost.StatusCode == http.StatusCreated {
							var createdEvt GoogleEventPayload
							if errDec := json.NewDecoder(respPost.Body).Decode(&createdEvt); errDec == nil {
								if regCol != nil {
									newReg := proxies.NewSyncRegistryRecord(regCol)
									newReg.SetUserID(authRecord.Id)
									newReg.SetEntityID(item.evt.Id)
									newReg.SetEntityType(proxies.SyncRegistryEntityTypeEvent)
									newReg.SetGoogleCalendarID(lassoCalID)
									newReg.SetGoogleEventID(createdEvt.ID)
									newReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
									newReg.SetStatus(proxies.SyncRegistryStatusActive)
									mu.Lock()
									evtRegsToSave = append(evtRegsToSave, newReg.Record)
									createdCount++
									mu.Unlock()
								}
							}
						} else {
							respB, _ := io.ReadAll(respPost.Body)
							log.Printf("[GoogleExport] Error creating event %s in Google (HTTP %d): %s", item.evt.Id, respPost.StatusCode, string(respB))
						}
						io.Copy(io.Discard, respPost.Body)
						respPost.Body.Close()
					}
				}
			}(de)
		}
		evtWg.Wait()

		if len(evtRegsToSave) > 0 {
			errBatch := app.RunInTransaction(func(txApp core.App) error {
				for _, rec := range evtRegsToSave {
					if err := txApp.Save(rec); err != nil {
						log.Printf("Error batch saving event sync registry record: %v", err)
					}
				}
				return nil
			})
			if errBatch != nil {
				log.Printf("Error committing event sync registry transaction: %v", errBatch)
			}
		}
	}

	log.Printf("[GoogleExport] Events evaluated: %d total (%d clean/skipped, %d dirty/dispatched)", len(validEvents), cleanEvtsCount, dirtyEvtsCount)

	totalSynced := createdCount + updatedCount
	msg := ""
	if totalSynced == 0 && deletedCount == 0 {
		msg = fmt.Sprintf("Coursework up to date (0 updates across %d courses).", len(coursesTaggedSet))
	} else {
		msg = fmt.Sprintf(
			"Synced %d coursework events to 'Lasso' calendar (%d created, %d updated across %d courses).",
			totalSynced, createdCount, updatedCount, len(coursesTaggedSet),
		)
		if deletedCount > 0 {
			msg = fmt.Sprintf(
				"Synced %d coursework events to 'Lasso' calendar (%d created, %d updated, %d events removed across %d courses).",
				totalSynced, createdCount, updatedCount, deletedCount, len(coursesTaggedSet),
			)
		}
	}

	_ = updateSyncStatusFinished(
		app,
		authRecord.Id,
		SyncOpGoogleExport,
		"success",
		msg,
		"",
		time.Since(startTime).Milliseconds(),
	)

	return &GoogleSyncResponse{
		Success:       true,
		CalendarID:    lassoCalID,
		SyncedCount:   totalSynced,
		CreatedCount:  createdCount,
		UpdatedCount:  updatedCount,
		DeletedCount:  deletedCount,
		CoursesTagged: len(coursesTaggedSet),
		Message:       msg,
	}, nil
}

func HandlePurgeLassoCalendar(app core.App) func(e *core.RequestEvent) error {
	return handlePurgeLassoCalendar(app)
}

func HandleSyncLassoToGoogle(app core.App) func(e *core.RequestEvent) error {
	return handleSyncLassoToGoogle(app)
}

func RunOutboundGoogleSync(app core.App, authRecord *core.Record) (*GoogleSyncResponse, error) {
	return runOutboundGoogleSync(app, authRecord)
}

func EnsureLassoGoogleCalendar(app core.App, authRecord *core.Record) (string, error) {
	return ensureLassoGoogleCalendar(app, authRecord)
}
