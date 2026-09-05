package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"backend/proxies"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Google Calendar Types & Models
// =============================================================================

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
	ColorID            string                         `json:"colorId,omitempty"`
	ExtendedProperties *GoogleEventExtendedProperties `json:"extendedProperties,omitempty"`
}

type GoogleEventListResponse struct {
	Items         []GoogleEventItem `json:"items"`
	NextPageToken string            `json:"nextPageToken,omitempty"`
}

type GoogleEventItem struct {
	ID                 string                         `json:"id"`
	Summary            string                         `json:"summary"`
	Description        string                         `json:"description"`
	Start              GoogleDateOrDateTime           `json:"start"`
	End                GoogleDateOrDateTime           `json:"end"`
	Status             string                         `json:"status"`
	ColorID            string                         `json:"colorId"`
	EventLabelID       string                         `json:"eventLabelId"`
	ExtendedProperties *GoogleEventExtendedProperties `json:"extendedProperties"`
}

type GoogleCalendarEntry struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

type EnsureLassoCalendarResponse struct {
	Success    bool   `json:"success"`
	CalendarID string `json:"calendarId"`
	Name       string `json:"name"`
	Message    string `json:"message,omitempty"`
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

type ReadOnlyEventsRequest struct {
	CalendarIDs []string `json:"calendarIds"`
	TimeMin     string   `json:"timeMin,omitempty"`
	TimeMax     string   `json:"timeMax,omitempty"`
}

type GoogleEventLabel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	BackgroundColor string `json:"backgroundColor"`
}

type ReadOnlyEventItem struct {
	ID           string `json:"id"`
	CalendarID   string `json:"calendarId"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	Start        string `json:"start"`
	End          string `json:"end"`
	AllDay       bool   `json:"allday"`
	Color        string `json:"color,omitempty"`
	EventLabelID string `json:"event_label_id,omitempty"`
	LabelName    string `json:"label_name,omitempty"`
	ReadOnly     bool   `json:"readOnly"`
}

type ReadOnlyEventsResponse struct {
	Success bool                `json:"success"`
	Events  []ReadOnlyEventItem `json:"events"`
	Count   int                 `json:"count"`
}

// 24 Calendar Color Palette defined by Google Calendar API
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

// fetchGoogleCalendarLabels fetches user-defined event labels from the calendar's configuration.
func fetchGoogleCalendarLabels(app core.App, authRecord *core.Record, calendarID string) map[string]GoogleEventLabel {
	labels := make(map[string]GoogleEventLabel)
	calURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s", url.PathEscape(calendarID))
	resp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", calURL, nil)
	if err != nil {
		return labels
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return labels
	}

	var calObj struct {
		LabelProperties struct {
			EventLabels []GoogleEventLabel `json:"eventLabels"`
		} `json:"labelProperties"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&calObj); err == nil {
		for _, lbl := range calObj.LabelProperties.EventLabels {
			if lbl.ID != "" {
				labels[lbl.ID] = lbl
			}
		}
	}
	return labels
}

type GoogleCalendarsRequest struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
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

// handleGoogleCalendars queries the Google Calendar API, applies nicknames, and returns calendars.
func handleGoogleCalendars(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		var req GoogleCalendarsRequest
		body, _ := io.ReadAll(e.Request.Body)
		if len(body) > 0 {
			_ = json.Unmarshal(body, &req)
		}

		// If frontend provided access / refresh tokens directly, save them
		if req.AccessToken != "" {
			authRecord.Set("google_access_token", req.AccessToken)
			authRecord.Set("google_connected", true)
		}
		if req.RefreshToken != "" {
			authRecord.Set("google_refresh_token", req.RefreshToken)
		}
		if req.AccessToken != "" || req.RefreshToken != "" {
			_ = app.Save(authRecord)
		}

		// Local calendar nicknames map from calendars collection
		localNicknames := map[string]string{}
		if authRecord != nil {
			calRecords, err := app.FindRecordsByFilter(
				"calendars",
				"user = {:user} && nickname != ''",
				"",
				200,
				0,
				map[string]any{"user": authRecord.Id},
			)
			if err == nil {
				for _, r := range calRecords {
					nick := r.GetString("nickname")
					if nick != "" {
						if cid := r.GetString("calendar_id"); cid != "" {
							localNicknames[cid] = nick
						}
						localNicknames[r.GetString("name")] = nick
						localNicknames[r.Id] = nick
					}
				}
			}
		}

		resp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", "https://www.googleapis.com/calendar/v3/users/me/calendarList", nil)
		if err != nil {
			return e.JSON(http.StatusOK, GetGoogleCalendarsResponse{
				Success:     false,
				Connected:   false,
				NeedsReauth: true,
				Email:       authRecord.GetString("email"),
				Message:     "Google Calendar credentials not found or expired. Please connect Google Calendar.",
			})
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return e.JSON(http.StatusOK, GetGoogleCalendarsResponse{
				Success:     false,
				Connected:   false,
				NeedsReauth: true,
				Email:       authRecord.GetString("email"),
				Message:     fmt.Sprintf("Google rejected credentials (HTTP %d). Please reconnect Google Calendar.", resp.StatusCode),
			})
		}

		var googleListResp struct {
			Items []map[string]any `json:"items"`
		}
		respBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(respBytes, &googleListResp); err != nil {
			return e.BadRequestError("Failed to parse Google Calendar response", err)
		}

		var calendars []map[string]any
		for _, item := range googleListResp.Items {
			cid := fmt.Sprintf("%v", item["id"])
			summary, _ := item["summary"].(string)
			if summary == "" {
				summary = cid
			}
			primary, _ := item["primary"].(bool)
			accessRole, _ := item["accessRole"].(string)

			category := "subscribed"
			if primary {
				category = "primary"
			} else if accessRole == "owner" || accessRole == "writer" {
				category = "owned"
			}

			colorHex := ""
			if bg, ok := item["backgroundColor"].(string); ok && strings.TrimSpace(bg) != "" {
				colorHex = strings.TrimSpace(bg)
			}
			colorIDVal := ""
			if cID, ok := item["colorId"].(string); ok && strings.TrimSpace(cID) != "" {
				colorIDVal = strings.TrimSpace(cID)
				if colorHex == "" {
					colorHex = googleCalendar24Palette[colorIDVal]
				}
			}
			if colorHex == "" {
				colorHex = "#515151"
			}

			entry := map[string]any{
				"id":              cid,
				"summary":         summary,
				"description":     item["description"],
				"timeZone":        item["timeZone"],
				"primary":         primary,
				"accessRole":      accessRole,
				"colorId":         colorIDVal,
				"backgroundColor": colorHex,
				"foregroundColor": item["foregroundColor"],
				"color":           colorHex,
				"category":        category,
			}

			if nick, ok := localNicknames[cid]; ok && nick != "" {
				entry["nickname"] = nick
			}

			// Sync color to PocketBase calendar record if it exists
			if authRecord != nil && colorHex != "" {
				if calRec, errRec := app.FindFirstRecordByFilter(
					"calendars",
					"user = {:user} && calendar_id = {:cid}",
					map[string]any{"user": authRecord.Id, "cid": cid},
				); errRec == nil && calRec != nil {
					if calRec.GetString("color") != colorHex {
						calRec.Set("color", colorHex)
						_ = app.Save(calRec)
					}
				}
			}

			calendars = append(calendars, entry)
		}

		authRecord.Set("google_connected", true)
		_ = app.Save(authRecord)

		userEmail := authRecord.GetString("google_email")
		if userEmail == "" {
			userEmail = authRecord.GetString("email")
		}

		return e.JSON(http.StatusOK, GetGoogleCalendarsResponse{
			Success:       true,
			Connected:     true,
			Email:         userEmail,
			CalendarCount: len(calendars),
			Calendars:     calendars,
			Message:       "Google calendars loaded successfully",
		})
	}
}

var courseCodeRegex = regexp.MustCompile(`(?i)\b([A-Z]{2,5})\s*([0-9]{3,4}[A-Z]?)\b`)

// extractCourseTag formats a short, readable course tag (e.g. "CS 1122")
func extractCourseTag(courseName string, nickname string) string {
	nick := strings.TrimSpace(nickname)
	if nick != "" {
		return nick
	}

	name := strings.TrimSpace(courseName)
	match := courseCodeRegex.FindStringSubmatch(name)
	if len(match) >= 3 {
		return fmt.Sprintf("%s %s", strings.ToUpper(match[1]), strings.ToUpper(match[2]))
	}

	// If no standard course code found, grab the first 2 words or up to 16 chars
	words := strings.Fields(name)
	if len(words) > 0 && len(words) <= 2 {
		return name
	}
	if len(words) >= 2 {
		short := fmt.Sprintf("%s %s", words[0], words[1])
		if len(short) <= 16 {
			return short
		}
	}
	if len(name) > 16 {
		return name[:16] + "..."
	}
	return name
}

// parseDateString handles standard ISO8601, RFC3339, and PocketBase space-separated dates
func parseDateString(dateStr string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return time.Time{}, false, fmt.Errorf("empty date string")
	}

	// Date-only: YYYY-MM-DD
	if len(trimmed) == 10 && !strings.Contains(trimmed, "T") && !strings.Contains(trimmed, " ") {
		t, err := time.Parse("2006-01-02", trimmed)
		return t, true, err
	}

	// Normalize space to T: e.g. "2026-08-31 03:59:00.000Z" -> "2026-08-31T03:59:00.000Z"
	normalized := strings.Replace(trimmed, " ", "T", 1)

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000Z",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, normalized); err == nil {
			return t, false, nil
		}
		if t, err := time.Parse(f, trimmed); err == nil {
			return t, false, nil
		}
	}

	return time.Time{}, false, fmt.Errorf("unable to parse date: %s", dateStr)
}

// =============================================================================
// Authenticated Google API Request Helper with Auto-Refresh
// =============================================================================

func makeGoogleAPIRequest(app core.App, authRecord *core.Record, method string, endpoint string, body []byte) (*http.Response, string, error) {
	token := authRecord.GetString("google_access_token")
	if token == "" {
		if authRecord.GetString("google_refresh_token") != "" {
			refreshed, err := refreshGoogleToken(app, authRecord)
			if err == nil && refreshed != "" {
				token = refreshed
			}
		}
	}
	if token == "" {
		return nil, "", fmt.Errorf("no valid Google OAuth token found")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, endpoint, bodyReader)
	if err != nil {
		return nil, token, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, token, err
	}

	// If token expired (401), attempt refresh and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		refreshed, errRef := refreshGoogleToken(app, authRecord)
		if errRef != nil || refreshed == "" {
			return nil, token, fmt.Errorf("google token expired and refresh failed: %v", errRef)
		}
		token = refreshed

		if len(body) > 0 {
			bodyReader = bytes.NewReader(body)
		}
		retryReq, err := http.NewRequest(method, endpoint, bodyReader)
		if err != nil {
			return nil, token, err
		}
		retryReq.Header.Set("Authorization", "Bearer "+token)
		retryReq.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			retryReq.Header.Set("Content-Type", "application/json")
		}

		resp, err = client.Do(retryReq)
		if err != nil {
			return nil, token, err
		}
	}

	return resp, token, nil
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
		if existingRecord.GetString("calendar_id") != foundGoogleCalID {
			existingRecord.Set("calendar_id", foundGoogleCalID)
			existingRecord.Set("source", "google")
			_ = app.Save(existingRecord)
		}
	} else {
		newCal := core.NewRecord(calendarsCollection)
		newCal.Set("user", authRecord.Id)
		newCal.Set("name", "Lasso")
		newCal.Set("color", "#515151")
		newCal.Set("source", "google")
		newCal.Set("visible", true)
		newCal.Set("calendar_id", foundGoogleCalID)
		if err := app.Save(newCal); err != nil {
			log.Printf("Warning: failed to save Lasso calendar record in PB: %v", err)
		}
	}

	return foundGoogleCalID, nil
}

// handleEnsureLassoCalendar ensures the dedicated Lasso calendar exists
func handleEnsureLassoCalendar(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		calID, err := ensureLassoGoogleCalendar(app, authRecord)
		if err != nil {
			return e.BadRequestError("Failed to ensure Lasso Google calendar: "+err.Error(), err)
		}

		return e.JSON(http.StatusOK, EnsureLassoCalendarResponse{
			Success:    true,
			CalendarID: calID,
			Name:       "Lasso",
			Message:    "Dedicated Lasso calendar is ready in Google Calendar.",
		})
	}
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

// syncLassoToGoogle pushes user coursework deadlines and events to the "Lasso" Google Calendar,
// tagging each event by course with title prefix [Course], description metadata, and color.
func syncLassoToGoogle(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		startTime := time.Now()
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		syncRec, err := getOrCreateSyncStatus(app, authRecord.Id)
		if err != nil {
			return e.InternalServerError("Failed to get sync status", err)
		}
		syncStatus := proxies.NewSyncStatus(syncRec)

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
			return e.JSON(http.StatusOK, GoogleSyncResponse{
				Success: true,
				Message: msg,
			})
		}

		_ = updateSyncStatusRunning(app, authRecord.Id, SyncOpGoogleExport)

		lassoCalID, err := ensureLassoGoogleCalendar(app, authRecord)
		if err != nil {
			_ = updateSyncStatusFinished(app, authRecord.Id, SyncOpGoogleExport, "error", "", err.Error(), time.Since(startTime).Milliseconds())
			return e.BadRequestError("Failed to setup Lasso Google calendar: "+err.Error(), err)
		}

		// 1. Fetch user course calendars for course tagging and color mapping
		courseCalendars, err := app.FindRecordsByFilter(
			"calendars",
			"user = {:user}",
			"name",
			200,
			0,
			map[string]any{"user": authRecord.Id},
		)
		if err != nil {
			return e.InternalServerError("Failed to fetch calendars", err)
		}

		calByID := make(map[string]*core.Record)
		for _, c := range courseCalendars {
			calByID[c.Id] = c
			if cid := c.GetString("course_id"); cid != "" {
				calByID[cid] = c
			}
		}

		// 2. Fetch all user tasks that have due dates
		tasks, err := app.FindRecordsByFilter(
			"tasks",
			"user = {:user} && due_date != ''",
			"due_date",
			500,
			0,
			map[string]any{"user": authRecord.Id},
		)
		if err != nil {
			return e.InternalServerError("Failed to fetch tasks", err)
		}

		validTaskIDs := make(map[string]bool)
		for _, task := range tasks {
			validTaskIDs[task.Id] = true
		}

		// 3. Fetch user coursework events / task work blocks from PocketBase events collection.
		// STRICT FILTER: NEVER sync events that originated from Google Calendar or belong to Google calendars!
		allEvents, _ := app.FindRecordsByFilter(
			"events",
			"google_event_id = '' && deadline != true",
			"start",
			500,
			0,
			nil,
		)

		var validEvents []*core.Record
		validEventIDs := make(map[string]bool)

		for _, evt := range allEvents {
			if evt.GetString("google_event_id") != "" {
				continue
			}

			calID := evt.GetString("calendar")
			var courseCal *core.Record = nil
			if calID != "" {
				courseCal = calByID[calID]
			}
			if courseCal == nil {
				continue
			}

			// Exclude any events belonging to Google calendars
			if courseCal.GetString("source") == "google" || courseCal.GetString("calendar_id") != "" {
				continue
			}

			// Must belong to this user
			if courseCal.GetString("user") != authRecord.Id {
				continue
			}

			// Must be a Canvas course event or a task work block
			if courseCal.GetString("source") != "canvas" && evt.GetString("task") == "" {
				continue
			}

			validEvents = append(validEvents, evt)
			validEventIDs[evt.Id] = true
		}

		// 4. Fetch existing managed events on the Google "Lasso" calendar
		// to make sync idempotent, update existing, and purge non-coursework / stray events.
		eventsURL := fmt.Sprintf(
			"https://www.googleapis.com/calendar/v3/calendars/%s/events?privateExtendedProperty=lasso_managed=true&maxResults=2500",
			url.PathEscape(lassoCalID),
		)
		existingResp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", eventsURL, nil)
		if err != nil {
			return e.BadRequestError("Failed to query Lasso calendar events: "+err.Error(), err)
		}
		defer existingResp.Body.Close()

		existingEventsByTaskID := make(map[string]string)  // task_id -> google_event_id
		existingEventsByEventID := make(map[string]string) // event_id -> google_event_id
		var itemsToDelete []string

		if existingResp.StatusCode == http.StatusOK {
			var listResp GoogleEventListResponse
			if err := json.NewDecoder(existingResp.Body).Decode(&listResp); err == nil {
				for _, itm := range listResp.Items {
					if itm.ExtendedProperties != nil && itm.ExtendedProperties.Private != nil {
						tid := itm.ExtendedProperties.Private["lasso_task_id"]
						eid := itm.ExtendedProperties.Private["lasso_event_id"]

						if tid != "" {
							if validTaskIDs[tid] {
								existingEventsByTaskID[tid] = itm.ID
							} else {
								itemsToDelete = append(itemsToDelete, itm.ID)
							}
						} else if eid != "" {
							if validEventIDs[eid] {
								existingEventsByEventID[eid] = itm.ID
							} else {
								itemsToDelete = append(itemsToDelete, itm.ID)
							}
						} else if itm.ExtendedProperties.Private["lasso_managed"] == "true" {
							itemsToDelete = append(itemsToDelete, itm.ID)
						}
					}
				}
			}
		}

		// Delete any stray or invalid events from the Google "Lasso" calendar
		deletedCount := 0
		if len(itemsToDelete) > 0 {
			var wg sync.WaitGroup
			sem := make(chan struct{}, 5)
			var mu sync.Mutex

			for _, itmID := range itemsToDelete {
				wg.Add(1)
				go func(id string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					delURL := fmt.Sprintf(
						"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s",
						url.PathEscape(lassoCalID),
						url.PathEscape(id),
					)
					delResp, _, errDel := makeGoogleAPIRequest(app, authRecord, "DELETE", delURL, nil)
					if errDel == nil {
						delResp.Body.Close()
						if delResp.StatusCode == http.StatusOK || delResp.StatusCode == http.StatusNoContent || delResp.StatusCode == http.StatusNotFound {
							mu.Lock()
							deletedCount++
							mu.Unlock()
						}
					}
				}(itmID)
			}
			wg.Wait()
			log.Printf("Cleaned up %d non-coursework/stray events from Google 'Lasso' calendar", deletedCount)
		}

		createdCount := 0
		updatedCount := 0
		coursesTaggedSet := make(map[string]bool)

		// 4. Iterate over tasks and upsert them as course-tagged events
		for _, task := range tasks {
			rawDue := task.GetString("due_date")
			if strings.TrimSpace(rawDue) == "" {
				rawDue = task.GetString("fake_due_date")
			}
			if strings.TrimSpace(rawDue) == "" {
				continue
			}

			// Resolve course calendar
			calID := task.GetString("calendar")
			var courseCal *core.Record = nil
			if calID != "" {
				courseCal = calByID[calID]
			}

			courseName := "Coursework"
			courseNickname := ""
			canvasCourseID := ""

			if courseCal != nil {
				courseName = courseCal.GetString("name")
				courseNickname = courseCal.GetString("nickname")
				canvasCourseID = courseCal.GetString("course_id")
			}

			// Tag course
			courseTag := extractCourseTag(courseName, courseNickname)
			coursesTaggedSet[courseTag] = true

			taskTitle := task.GetString("name")
			// Add course prefix to event title: [CS 1122] Homework 1
			taggedSummary := fmt.Sprintf("[%s] %s", courseTag, taskTitle)
			if task.GetString("status") == "done" {
				taggedSummary = fmt.Sprintf("[%s] ✓ %s", courseTag, taskTitle)
			}

			// Determine start and end using robust date parser
			parsedTime, isDateOnly, err := parseDateString(rawDue)
			if err != nil {
				log.Printf("Skipping task %s (%s) due to unparseable date '%s': %v", task.Id, taskTitle, rawDue, err)
				continue
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

			// Rich description
			statusText := "To Do"
			if task.GetString("status") == "done" {
				statusText = "Completed"
			}
			priorityText := strings.ToUpper(task.GetString("priority"))
			if priorityText == "" {
				priorityText = "NORMAL"
			}

			description := fmt.Sprintf(
				"Course: %s\nStatus: %s\nPriority: %s\nDue: %s\n\nSynced by Lasso",
				courseName,
				statusText,
				priorityText,
				rawDue,
			)

			payload := GoogleEventPayload{
				Summary:     taggedSummary,
				Description: description,
				Start:       startObj,
				End:         endObj,
				ExtendedProperties: &GoogleEventExtendedProperties{
					Private: map[string]string{
						"lasso_managed":     "true",
						"lasso_task_id":     task.Id,
						"lasso_course_id":   canvasCourseID,
						"lasso_course_name": courseName,
						"lasso_tag":         courseTag,
					},
				},
			}

			payloadBytes, _ := json.Marshal(payload)

			if existingGEventID, exists := existingEventsByTaskID[task.Id]; exists {
				// Update existing event (PATCH)
				patchURL := fmt.Sprintf(
					"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s",
					url.PathEscape(lassoCalID),
					url.PathEscape(existingGEventID),
				)
				resp, _, err := makeGoogleAPIRequest(app, authRecord, "PATCH", patchURL, payloadBytes)
				if err != nil {
					log.Printf("Failed to PATCH event for task %s: %v", task.Id, err)
				} else {
					if resp.StatusCode == http.StatusOK {
						updatedCount++
					} else {
						b, _ := io.ReadAll(resp.Body)
						log.Printf("Google PATCH returned status %d for task %s: %s", resp.StatusCode, task.Id, string(b))
					}
					resp.Body.Close()
				}
			} else {
				// Insert new event (POST)
				insertURL := fmt.Sprintf(
					"https://www.googleapis.com/calendar/v3/calendars/%s/events",
					url.PathEscape(lassoCalID),
				)
				resp, _, err := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
				if err != nil {
					log.Printf("Failed to POST event for task %s: %v", task.Id, err)
				} else {
					if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
						createdCount++
					} else {
						b, _ := io.ReadAll(resp.Body)
						log.Printf("Google POST returned status %d for task %s: %s", resp.StatusCode, task.Id, string(b))
					}
					resp.Body.Close()
				}
			}
		}

		// 5. Iterate over valid coursework events / work sessions and upsert them
		for _, evt := range validEvents {
			rawStart := evt.GetString("start")
			if strings.TrimSpace(rawStart) == "" {
				continue
			}

			calID := evt.GetString("calendar")
			var courseCal *core.Record = nil
			if calID != "" {
				courseCal = calByID[calID]
			}
			if courseCal == nil && calID != "" {
				continue
			}

			courseName := "Coursework"
			courseNickname := ""
			canvasCourseID := ""
			if courseCal != nil {
				courseName = courseCal.GetString("name")
				courseNickname = courseCal.GetString("nickname")
				canvasCourseID = courseCal.GetString("course_id")
			}

			courseTag := extractCourseTag(courseName, courseNickname)
			coursesTaggedSet[courseTag] = true

			evtTitle := evt.GetString("title")
			if evtTitle == "" {
				evtTitle = "Event"
			}
			taggedSummary := fmt.Sprintf("[%s] %s", courseTag, evtTitle)

			startParsed, isStartOnlyDate, errStart := parseDateString(rawStart)
			if errStart != nil {
				log.Printf("Skipping event %s due to unparseable start date '%s': %v", evt.Id, rawStart, errStart)
				continue
			}

			var startObj, endObj GoogleDateOrDateTime
			if evt.GetBool("allday") || isStartOnlyDate {
				dateStr := startParsed.Format("2006-01-02")
				startObj = GoogleDateOrDateTime{Date: dateStr}
				rawEnd := evt.GetString("end")
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
				rawEnd := evt.GetString("end")
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

			payload := GoogleEventPayload{
				Summary:     taggedSummary,
				Description: fmt.Sprintf("Course: %s\n\nSynced by Lasso", courseName),
				Start:       startObj,
				End:         endObj,
				ExtendedProperties: &GoogleEventExtendedProperties{
					Private: map[string]string{
						"lasso_managed":     "true",
						"lasso_event_id":    evt.Id,
						"lasso_course_id":   canvasCourseID,
						"lasso_course_name": courseName,
						"lasso_tag":         courseTag,
					},
				},
			}

			payloadBytes, _ := json.Marshal(payload)

			if existingGEventID, exists := existingEventsByEventID[evt.Id]; exists {
				patchURL := fmt.Sprintf(
					"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s",
					url.PathEscape(lassoCalID),
					url.PathEscape(existingGEventID),
				)
				resp, _, err := makeGoogleAPIRequest(app, authRecord, "PATCH", patchURL, payloadBytes)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						updatedCount++
					}
				}
			} else {
				insertURL := fmt.Sprintf(
					"https://www.googleapis.com/calendar/v3/calendars/%s/events",
					url.PathEscape(lassoCalID),
				)
				resp, _, err := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
						createdCount++
					}
				}
			}
		}

		totalSynced := createdCount + updatedCount

		msg := fmt.Sprintf(
			"Synced %d coursework events to 'Lasso' calendar (%d created, %d updated across %d courses).",
			totalSynced, createdCount, updatedCount, len(coursesTaggedSet),
		)
		if deletedCount > 0 {
			msg = fmt.Sprintf(
				"Synced %d coursework events to 'Lasso' calendar (%d created, %d updated, %d non-coursework events removed across %d courses).",
				totalSynced, createdCount, updatedCount, deletedCount, len(coursesTaggedSet),
			)
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

		return e.JSON(http.StatusOK, GoogleSyncResponse{
			Success:       true,
			CalendarID:    lassoCalID,
			SyncedCount:   totalSynced,
			CreatedCount:  createdCount,
			UpdatedCount:  updatedCount,
			DeletedCount:  deletedCount,
			CoursesTagged: len(coursesTaggedSet),
			Message:       msg,
		})
	}
}

// =============================================================================
// Read-Only Fetch for User's Personal Calendars
// =============================================================================

// fetchReadOnlyGoogleEvents retrieves events from user's personal Google calendars
// strictly for viewing on Lasso's calendar grid. Never modifies personal calendars.
func fetchReadOnlyGoogleEvents(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		var req ReadOnlyEventsRequest
		body, _ := io.ReadAll(e.Request.Body)
		if len(body) > 0 {
			_ = json.Unmarshal(body, &req)
		}

		if len(req.CalendarIDs) == 0 {
			return e.JSON(http.StatusOK, ReadOnlyEventsResponse{
				Success: true,
				Events:  []ReadOnlyEventItem{},
				Count:   0,
			})
		}

		nowUTC := time.Now().UTC()
		timeMin := req.TimeMin
		if timeMin == "" {
			timeMin = nowUTC.AddDate(0, -1, 0).Format(time.RFC3339)
		}
		timeMax := req.TimeMax
		if timeMax == "" {
			timeMax = nowUTC.AddDate(0, 3, 0).Format(time.RFC3339)
		}

		var collectedEvents []ReadOnlyEventItem
		seenCalIDs := make(map[string]bool)
		seenEventKeys := make(map[string]bool)

		// Query each requested personal calendar (strictly read-only)
		for _, calID := range req.CalendarIDs {
			trimmedID := strings.TrimSpace(calID)
			if trimmedID == "" || seenCalIDs[trimmedID] {
				continue
			}
			seenCalIDs[trimmedID] = true

			// Resolve calendar's default color from PocketBase if available
			defaultCalColor := "#515151"
			if pbCal, errPB := app.FindFirstRecordByFilter(
				"calendars",
				"user = {:user} && calendar_id = {:cid}",
				map[string]any{"user": authRecord.Id, "cid": trimmedID},
			); errPB == nil && pbCal != nil {
				if c := pbCal.GetString("color"); c != "" {
					defaultCalColor = c
				}
			}

			// Fetch user-defined event labels from the calendar's configuration
			labelColors := fetchGoogleCalendarLabels(app, authRecord, trimmedID)

			apiURL := fmt.Sprintf(
				"https://www.googleapis.com/calendar/v3/calendars/%s/events?singleEvents=true&orderBy=startTime&timeMin=%s&timeMax=%s&maxResults=250&eventLabelVersion=1",
				url.PathEscape(trimmedID),
				url.QueryEscape(timeMin),
				url.QueryEscape(timeMax),
			)

			resp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", apiURL, nil)
			if err != nil {
				log.Printf("Failed to fetch read-only events from cal %s: %v", trimmedID, err)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				continue
			}

			var listResp GoogleEventListResponse
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()

			for _, itm := range listResp.Items {
				// Skip transparent / cancelled
				if itm.Status == "cancelled" {
					continue
				}

				// Skip Lasso-managed events to prevent echoing our own exported items
				if itm.ExtendedProperties != nil && itm.ExtendedProperties.Private["lasso_managed"] == "true" {
					continue
				}

				startVal := itm.Start.DateTime
				allDay := false
				if startVal == "" {
					startVal = itm.Start.Date
					allDay = true
				}

				endVal := itm.End.DateTime
				if endVal == "" {
					endVal = itm.End.Date
				}

				eventKey := fmt.Sprintf("%s:%s:%s", trimmedID, itm.ID, startVal)
				if seenEventKeys[eventKey] {
					continue
				}
				seenEventKeys[eventKey] = true

				eventColor := defaultCalColor
				labelName := ""
				if itm.EventLabelID != "" {
					if lbl, found := labelColors[itm.EventLabelID]; found {
						if lbl.BackgroundColor != "" {
							eventColor = lbl.BackgroundColor
						}
						labelName = lbl.Name
					}
				}

				collectedEvents = append(collectedEvents, ReadOnlyEventItem{
					ID:           itm.ID,
					CalendarID:   trimmedID,
					Title:        itm.Summary,
					Description:  itm.Description,
					Start:        startVal,
					End:          endVal,
					AllDay:       allDay,
					Color:        eventColor,
					EventLabelID: itm.EventLabelID,
					LabelName:    labelName,
					ReadOnly:     true,
				})
			}
		}

		return e.JSON(http.StatusOK, ReadOnlyEventsResponse{
			Success: true,
			Events:  collectedEvents,
			Count:   len(collectedEvents),
		})
	}
}

// InboundSyncResponse defines the payload returned after an inbound Google sync
type InboundSyncResponse struct {
	Success         bool   `json:"success"`
	CalendarsSynced int    `json:"calendarsSynced"`
	EventsSynced    int    `json:"eventsSynced"`
	EventsDeleted   int    `json:"eventsDeleted"`
	Message         string `json:"message"`
}

// syncInboundGoogleCalendars handles the POST /api/google/sync-inbound endpoint
func syncInboundGoogleCalendars(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		startTime := time.Now()
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		_ = updateSyncStatusRunning(app, authRecord.Id, SyncOpGoogleImport)

		res, err := runInboundGoogleSync(app, authRecord)
		if err != nil {
			_ = updateSyncStatusFinished(app, authRecord.Id, SyncOpGoogleImport, "error", "", err.Error(), time.Since(startTime).Milliseconds())
			return e.BadRequestError(fmt.Sprintf("Failed to sync from Google Calendar: %v", err), nil)
		}

		_ = updateSyncStatusFinished(app, authRecord.Id, SyncOpGoogleImport, "success", res.Message, "", time.Since(startTime).Milliseconds())

		return e.JSON(http.StatusOK, res)
	}
}

// runInboundGoogleSync queries Google Calendar API and persists calendars and events into PocketBase
func runInboundGoogleSync(app core.App, authRecord *core.Record) (*InboundSyncResponse, error) {
	if authRecord == nil {
		return nil, fmt.Errorf("authentication required")
	}

	calendarsCol, err := app.FindCollectionByNameOrId("calendars")
	if err != nil {
		return nil, fmt.Errorf("calendars collection not found: %w", err)
	}
	eventsCol, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		return nil, fmt.Errorf("events collection not found: %w", err)
	}

	// Fetch user's Google Calendars
	resp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", "https://www.googleapis.com/calendar/v3/users/me/calendarList", nil)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google calendars: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Google API error fetching calendars (HTTP %d)", resp.StatusCode)
	}

	var calListResp struct {
		Items []struct {
			ID              string `json:"id"`
			Summary         string `json:"summary"`
			Description     string `json:"description"`
			ColorID         string `json:"colorId"`
			BackgroundColor string `json:"backgroundColor"`
			Primary         bool   `json:"primary"`
			AccessRole      string `json:"accessRole"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&calListResp); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to decode Google calendars: %w", err)
	}
	resp.Body.Close()

	calendarsSynced := 0
	eventsSynced := 0
	eventsDeleted := 0

	nowUTC := time.Now().UTC()
	timeMin := nowUTC.AddDate(0, -1, 0).Format(time.RFC3339)
	timeMax := nowUTC.AddDate(0, 3, 0).Format(time.RFC3339)

	// Map Google Calendar ID -> PocketBase Calendar Record ID
	gCalToLocalID := make(map[string]string)
	calsToSyncEvents := make([]string, 0)

	for _, gcal := range calListResp.Items {
		cid := strings.TrimSpace(gcal.ID)
		if cid == "" {
			continue
		}

		summary := strings.TrimSpace(gcal.Summary)
		if summary == "" {
			summary = cid
		}

		isLassoCal := strings.EqualFold(summary, "Lasso") || strings.Contains(strings.ToLower(gcal.Description), "lasso")

		// Find existing calendar in PocketBase
		existingCal, _ := app.FindFirstRecordByFilter(
			"calendars",
			"user = {:user} && calendar_id = {:calId}",
			map[string]any{
				"user":  authRecord.Id,
				"calId": cid,
			},
		)
		calColor := strings.TrimSpace(gcal.BackgroundColor)
		if calColor == "" && gcal.ColorID != "" {
			calColor = googleCalendar24Palette[strings.TrimSpace(gcal.ColorID)]
		}
		if calColor == "" {
			calColor = "#515151"
		}

		targetCal := existingCal
		if targetCal == nil {
			targetCal = core.NewRecord(calendarsCol)
			targetCal.Set("user", authRecord.Id)
			targetCal.Set("source", "google")
			targetCal.Set("calendar_id", cid)
			targetCal.Set("name", summary)
			targetCal.Set("color", calColor)
			targetCal.Set("visible", true)
		} else {
			// Update color to match Google's latest palette color
			if calColor != "" && targetCal.GetString("color") != calColor {
				targetCal.Set("color", calColor)
			}
			// Update name if changed and no custom nickname set
			if targetCal.GetString("nickname") == "" && targetCal.GetString("name") != summary {
				targetCal.Set("name", summary)
			}
		}

		if err := app.Save(targetCal); err == nil {
			gCalToLocalID[cid] = targetCal.Id
			calendarsSynced++
			if !isLassoCal {
				calsToSyncEvents = append(calsToSyncEvents, cid)
			}
		} else {
			log.Printf("Warning: failed to save calendar %s: %v", summary, err)
		}
	}

	// Now sync events for each personal calendar
	for _, gcalID := range calsToSyncEvents {
		localCalID := gCalToLocalID[gcalID]
		if localCalID == "" {
			continue
		}

		// Find target calendar record in PocketBase to get its default color
		calRecord, _ := app.FindRecordById("calendars", localCalID)
		defaultCalColor := "#515151"
		if calRecord != nil && calRecord.GetString("color") != "" {
			defaultCalColor = calRecord.GetString("color")
		}

		// Fetch user-defined event labels from the calendar's configuration
		labelColors := fetchGoogleCalendarLabels(app, authRecord, gcalID)

		apiURL := fmt.Sprintf(
			"https://www.googleapis.com/calendar/v3/calendars/%s/events?singleEvents=true&orderBy=startTime&timeMin=%s&timeMax=%s&maxResults=250&eventLabelVersion=1",
			url.PathEscape(gcalID),
			url.QueryEscape(timeMin),
			url.QueryEscape(timeMax),
		)

		evResp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", apiURL, nil)
		if err != nil {
			log.Printf("Warning: failed to fetch events for cal %s: %v", gcalID, err)
			continue
		}
		if evResp.StatusCode != http.StatusOK {
			evResp.Body.Close()
			continue
		}

		var listResp GoogleEventListResponse
		if err := json.NewDecoder(evResp.Body).Decode(&listResp); err != nil {
			evResp.Body.Close()
			continue
		}
		evResp.Body.Close()

		activeGoogleIDs := make(map[string]bool)

		for _, itm := range listResp.Items {
			if itm.Status == "cancelled" {
				continue
			}

			startVal := itm.Start.DateTime
			allDay := false
			if startVal == "" {
				startVal = itm.Start.Date
				allDay = true
			}

			endVal := itm.End.DateTime
			if endVal == "" {
				endVal = itm.End.Date
			}

			startT, _, errStart := parseDateString(startVal)
			if errStart != nil {
				continue
			}
			endT, _, errEnd := parseDateString(endVal)
			if errEnd != nil || endT.Before(startT) {
				endT = startT.Add(1 * time.Hour)
			}

			eventColor := defaultCalColor
			labelName := ""
			eventLabelID := itm.EventLabelID
			if eventLabelID != "" {
				if lbl, found := labelColors[eventLabelID]; found {
					if lbl.BackgroundColor != "" {
						eventColor = lbl.BackgroundColor
					}
					labelName = lbl.Name
				}
			}

			title := strings.TrimSpace(itm.Summary)
			if title == "" {
				title = "(No title)"
			}

			activeGoogleIDs[itm.ID] = true

			// Look for existing event in PocketBase
			existingEvt, _ := app.FindFirstRecordByFilter(
				"events",
				"calendar = {:calId} && google_event_id = {:gid}",
				map[string]any{
					"calId": localCalID,
					"gid":   itm.ID,
				},
			)

			targetEvt := existingEvt
			if targetEvt == nil {
				targetEvt = core.NewRecord(eventsCol)
				targetEvt.Set("calendar", localCalID)
				targetEvt.Set("google_event_id", itm.ID)
				targetEvt.Set("deadline", false)
			}

			targetEvt.Set("title", title)
			targetEvt.Set("start", startT.UTC().Format("2006-01-02 15:04:05.000Z"))
			targetEvt.Set("end", endT.UTC().Format("2006-01-02 15:04:05.000Z"))
			targetEvt.Set("allday", allDay)
			targetEvt.Set("color", eventColor)
			targetEvt.Set("event_label_id", eventLabelID)
			targetEvt.Set("label_name", labelName)
			if itm.Description != "" {
				targetEvt.Set("description", itm.Description)
			}

			if err := app.Save(targetEvt); err == nil {
				eventsSynced++
			} else {
				log.Printf("Warning: failed to save event %s: %v", title, err)
			}
		}

		// Clean up deleted events in PocketBase for this calendar within the sync window
		localEvents, err := app.FindRecordsByFilter(
			"events",
			"calendar = {:calId} && google_event_id != ''",
			"",
			500,
			0,
			map[string]any{
				"calId": localCalID,
			},
		)
		if err == nil {
			for _, rec := range localEvents {
				gid := rec.GetString("google_event_id")
				if gid == "" {
					continue
				}
				recStart := rec.GetString("start")
				if recStartT, _, err := parseDateString(recStart); err == nil {
					if (recStartT.After(nowUTC.AddDate(0, -1, 0)) || recStartT.Equal(nowUTC.AddDate(0, -1, 0))) &&
						(recStartT.Before(nowUTC.AddDate(0, 3, 0)) || recStartT.Equal(nowUTC.AddDate(0, 3, 0))) {
						if !activeGoogleIDs[gid] {
							if err := app.Delete(rec); err == nil {
								eventsDeleted++
							}
						}
					}
				}
			}
		}
	}

	return &InboundSyncResponse{
		Success:         true,
		CalendarsSynced: calendarsSynced,
		EventsSynced:    eventsSynced,
		EventsDeleted:   eventsDeleted,
		Message:         fmt.Sprintf("Successfully synced %d calendars and %d events from Google Calendar.", calendarsSynced, eventsSynced),
	}, nil
}
