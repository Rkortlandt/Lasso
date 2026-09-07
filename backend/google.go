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

	"github.com/google/uuid"
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
	EventLabelID       string                         `json:"eventLabelId,omitempty"`
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

// getCalendarGoogleLabelID returns a deterministic RFC 4122 UUID v5 for a calendar.
func getCalendarGoogleLabelID(calID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("lasso:calendar:"+calID)).String()
}

// reconcileCalendarTagsIfDirty checks if user labels or coursework calendars were updated
// since the last tag sync. If unchanged, it makes 0 Google API calls.
// If changed, it updates the Google Calendar's labelProperties.eventLabels with 1 PATCH request.
func reconcileCalendarTagsIfDirty(
	app core.App,
	authRecord *core.Record,
	calendarID string,
	syncStatus *proxies.SyncStatus,
	courseCalendars []*proxies.Calendar,
	userLabels []*proxies.Label,
) error {
	lastTagSyncStr := syncStatus.GoogleTagsSyncedAt()
	needsUpdate := false

	if lastTagSyncStr == "" {
		needsUpdate = true
	} else {
		lastTagSync, err := time.Parse(time.RFC3339, lastTagSyncStr)
		if err != nil {
			needsUpdate = true
		} else {
			for _, cal := range courseCalendars {
				if cal.GetDateTime("updated").Time().After(lastTagSync) {
					needsUpdate = true
					break
				}
			}
			if !needsUpdate {
				for _, lbl := range userLabels {
					if lbl.GetDateTime("updated").Time().After(lastTagSync) {
						needsUpdate = true
						break
					}
				}
			}
		}
	}

	if !needsUpdate {
		return nil // 0 Google API calls!
	}

	labelByID := make(map[string]*proxies.Label)
	for _, lbl := range userLabels {
		labelByID[lbl.Id] = lbl
	}

	var updatedEventLabels []GoogleEventLabel
	for _, cal := range courseCalendars {
		if cal.Source() != proxies.CalendarSourceCanvas && cal.Source() != proxies.CalendarSourceInternal && cal.Source() != proxies.CalendarSourceTodo {
			continue
		}

		gid := strings.TrimSpace(cal.GoogleLabelID())
		if gid == "" {
			gid = getCalendarGoogleLabelID(cal.Id)
			cal.SetGoogleLabelID(gid)
			if err := app.Save(cal); err != nil {
				log.Printf("Warning: failed to save google_label_id for calendar %s: %v", cal.Id, err)
			}
		}

		labelName := ""
		labelColor := cal.Color()
		if cal.LabelID() != "" {
			if lbl := labelByID[cal.LabelID()]; lbl != nil {
				if lbl.Name() != "" {
					labelName = lbl.Name()
				}
				if lbl.Color() != "" {
					labelColor = lbl.Color()
				}
			}
		}
		if labelName == "" {
			labelName = extractCourseTag(cal.Name(), cal.Nickname())
		}
		if labelColor == "" {
			labelColor = "#2563eb"
		}

		updatedEventLabels = append(updatedEventLabels, GoogleEventLabel{
			ID:              gid,
			Name:            labelName,
			BackgroundColor: labelColor,
		})
	}

	patchPayload := map[string]any{
		"labelProperties": map[string]any{
			"eventLabels": updatedEventLabels,
		},
	}
	body, err := json.Marshal(patchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal labelProperties: %w", err)
	}

	calURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s", url.PathEscape(calendarID))
	resp, _, err := makeGoogleAPIRequest(app, authRecord, "PATCH", calURL, body)
	if err != nil {
		return fmt.Errorf("failed to patch calendar labelProperties: %w", err)
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("PATCH calendar labelProperties failed (%d): %s", resp.StatusCode, string(bodyBytes))
		}
	}

	syncStatus.SetGoogleTagsSyncedAt(time.Now().UTC().Format(time.RFC3339))
	_ = app.Save(syncStatus)
	log.Printf("Successfully updated %d static calendar labels on Google calendar %s", len(updatedEventLabels), calendarID)
	return nil
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
				proxies.CollectionCalendars,
				"user = {:user} && nickname != ''",
				"",
				200,
				0,
				map[string]any{"user": authRecord.Id},
			)
			if err == nil {
				for _, r := range calRecords {
					cal := proxies.NewCalendar(r)
					nick := cal.Nickname()
					if nick != "" {
						if cid := cal.CalendarID(); cid != "" {
							localNicknames[cid] = nick
						}
						localNicknames[cal.Name()] = nick
						localNicknames[cal.Id] = nick
					}
				}
			}
		}

		authUser := proxies.NewUser(authRecord)

		resp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", "https://www.googleapis.com/calendar/v3/users/me/calendarList", nil)
		if err != nil {
			return e.JSON(http.StatusOK, GetGoogleCalendarsResponse{
				Success:     false,
				Connected:   false,
				NeedsReauth: true,
				Email:       authUser.Email(),
				Message:     "Google Calendar credentials not found or expired. Please connect Google Calendar.",
			})
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return e.JSON(http.StatusOK, GetGoogleCalendarsResponse{
				Success:     false,
				Connected:   false,
				NeedsReauth: true,
				Email:       authUser.Email(),
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
					proxies.CollectionCalendars,
					"user = {:user} && calendar_id = {:cid}",
					map[string]any{"user": authRecord.Id, "cid": cid},
				); errRec == nil && calRec != nil {
					cal := proxies.NewCalendar(calRec)
					if cal.Color() != colorHex {
						cal.SetColor(colorHex)
						_ = app.Save(cal)
					}
				}
			}

			calendars = append(calendars, entry)
		}

		authUser.SetGoogleConnected(true)
		_ = app.Save(authUser)

		userEmail := authUser.GoogleEmail()
		if userEmail == "" {
			userEmail = authUser.Email()
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
// Authenticated Google API Request Helper with Auto-Refresh & HTTP/2 Pooling
// =============================================================================

var (
	googleSharedHTTPClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
		},
		Timeout: 25 * time.Second,
	}
	googleTokenRefreshMu sync.Mutex
	googleWriteLimiter   = time.NewTicker(time.Second / 4) // Strict cap: 4 writes / sec
)

func makeGoogleAPIRequest(app core.App, authRecord *core.Record, method string, endpoint string, body []byte) (*http.Response, string, error) {
	authUser := proxies.NewUser(authRecord)
	token := authUser.GoogleAccessToken()
	if token == "" {
		if authUser.GoogleRefreshToken() != "" {
			googleTokenRefreshMu.Lock()
			refreshed, err := refreshGoogleToken(app, authRecord)
			googleTokenRefreshMu.Unlock()
			if err == nil && refreshed != "" {
				token = refreshed
			}
		}
	}
	if token == "" {
		return nil, "", fmt.Errorf("no valid Google OAuth token found")
	}

	// Enforce strict rate limit on write requests (POST, PATCH, DELETE, PUT): max 9 writes / second
	if method != http.MethodGet && method != http.MethodHead {
		<-googleWriteLimiter.C
	}

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

	resp, err := googleSharedHTTPClient.Do(req)
	if err != nil {
		return nil, token, err
	}

	// If token expired (401), attempt refresh and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		googleTokenRefreshMu.Lock()
		refreshedUser := proxies.NewUser(authRecord)
		currentToken := refreshedUser.GoogleAccessToken()
		if currentToken != "" && currentToken != token {
			token = currentToken
			googleTokenRefreshMu.Unlock()
		} else {
			refreshed, errRef := refreshGoogleToken(app, authRecord)
			googleTokenRefreshMu.Unlock()
			if errRef != nil || refreshed == "" {
				return nil, token, fmt.Errorf("google token expired and refresh failed: %v", errRef)
			}
			token = refreshed
		}

		if method != http.MethodGet && method != http.MethodHead {
			<-googleWriteLimiter.C
		}

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

		resp, err = googleSharedHTTPClient.Do(retryReq)
		if err != nil {
			return nil, token, err
		}
	}

	// If rate limited (429 or 403 quota/rateLimitExceeded), back off and retry up to 3 times
	for attempt := 1; attempt <= 3 && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden); attempt++ {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		backoff := time.Duration(attempt) * 1000 * time.Millisecond
		log.Printf("[RateLimit] Google API rate limited (%d). Backing off %v (attempt %d/3)...", resp.StatusCode, backoff, attempt)
		time.Sleep(backoff)

		if method != http.MethodGet && method != http.MethodHead {
			<-googleWriteLimiter.C
		}

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

		resp, err = googleSharedHTTPClient.Do(retryReq)
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
		allEvents, _ := app.FindRecordsByFilter(
			proxies.CollectionEvents,
			"google_event_id = '' && deadline != true",
			"start",
			500,
			0,
			nil,
		)

		var validEvents []*proxies.Event
		for _, rawEvt := range allEvents {
			evt := proxies.NewEvent(rawEvt)
			if evt.GoogleEventID() != "" {
				continue
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
			if courseCal.Source() != proxies.CalendarSourceCanvas && evt.TaskID() == "" {
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
						if eventLabelID == "" {
							eventLabelID = getCalendarGoogleLabelID(item.courseCal.Id)
						}
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

					if item.existingReg != nil && item.existingReg.GoogleEventID() != "" {
						// Update existing event (PATCH)
						patchURL := fmt.Sprintf(
							"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s?eventLabelVersion=1",
							url.PathEscape(lassoCalID),
							url.PathEscape(item.existingReg.GoogleEventID()),
						)
						resp, _, errPatch := makeGoogleAPIRequest(app, authRecord, "PATCH", patchURL, payloadBytes)
						if errPatch == nil {
							if resp.StatusCode == http.StatusOK {
								item.existingReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
								mu.Lock()
								taskRegsToSave = append(taskRegsToSave, item.existingReg.Record)
								updatedCount++
								mu.Unlock()
							} else if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
								// Re-create via POST if Google event was deleted remotely
								insertURL := fmt.Sprintf(
									"https://www.googleapis.com/calendar/v3/calendars/%s/events?eventLabelVersion=1",
									url.PathEscape(lassoCalID),
								)
								respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
								if errPost == nil {
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
									}
									io.Copy(io.Discard, respPost.Body)
									respPost.Body.Close()
								}
							}
							io.Copy(io.Discard, resp.Body)
							resp.Body.Close()
						}
					} else {
						// Create new event (POST)
						insertURL := fmt.Sprintf(
							"https://www.googleapis.com/calendar/v3/calendars/%s/events?eventLabelVersion=1",
							url.PathEscape(lassoCalID),
						)
						respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
						if errPost == nil {
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
						if eventLabelID == "" {
							eventLabelID = getCalendarGoogleLabelID(item.courseCal.Id)
						}
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
					payloadBytes, _ := json.Marshal(payload)

					if item.existingReg != nil && item.existingReg.GoogleEventID() != "" {
						patchURL := fmt.Sprintf(
							"https://www.googleapis.com/calendar/v3/calendars/%s/events/%s?eventLabelVersion=1",
							url.PathEscape(lassoCalID),
							url.PathEscape(item.existingReg.GoogleEventID()),
						)
						resp, _, errPatch := makeGoogleAPIRequest(app, authRecord, "PATCH", patchURL, payloadBytes)
						if errPatch == nil {
							if resp.StatusCode == http.StatusOK {
								item.existingReg.SetSyncedAt(time.Now().UTC().Format("2006-01-02 15:04:05.000Z"))
								mu.Lock()
								evtRegsToSave = append(evtRegsToSave, item.existingReg.Record)
								updatedCount++
								mu.Unlock()
							} else if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
								// Re-create via POST if Google event was deleted remotely
								insertURL := fmt.Sprintf(
									"https://www.googleapis.com/calendar/v3/calendars/%s/events?eventLabelVersion=1",
									url.PathEscape(lassoCalID),
								)
								respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
								if errPost == nil {
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
									}
									io.Copy(io.Discard, respPost.Body)
									respPost.Body.Close()
								}
							}
							io.Copy(io.Discard, resp.Body)
							resp.Body.Close()
						}
					} else {
						insertURL := fmt.Sprintf(
							"https://www.googleapis.com/calendar/v3/calendars/%s/events?eventLabelVersion=1",
							url.PathEscape(lassoCalID),
						)
						respPost, _, errPost := makeGoogleAPIRequest(app, authRecord, "POST", insertURL, payloadBytes)
						if errPost == nil {
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
				if itm.ExtendedProperties != nil && itm.ExtendedProperties.Private != nil && itm.ExtendedProperties.Private["lasso_managed"] == "true" {
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
	EventsCreated   int    `json:"eventsCreated"`
	EventsUpdated   int    `json:"eventsUpdated"`
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
	eventsCreated := 0
	eventsUpdated := 0
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

		var targetCal *proxies.Calendar
		needsCalSave := false
		if existingCal == nil {
			targetCal = proxies.NewCalendarRecord(calendarsCol)
			targetCal.SetUserID(authRecord.Id)
			targetCal.SetSource(proxies.CalendarSourceGoogle)
			targetCal.SetCalendarID(cid)
			targetCal.SetName(summary)
			targetCal.SetColor(calColor)
			targetCal.SetVisible(true)
			needsCalSave = true
		} else {
			targetCal = proxies.NewCalendar(existingCal)
			// Update color to match Google's latest palette color
			if calColor != "" && targetCal.Color() != calColor {
				targetCal.SetColor(calColor)
				needsCalSave = true
			}
			// Update name if changed and no custom nickname set
			if targetCal.Nickname() == "" && targetCal.Name() != summary {
				targetCal.SetName(summary)
				needsCalSave = true
			}
		}

		if needsCalSave {
			if err := app.Save(targetCal); err == nil {
				gCalToLocalID[cid] = targetCal.Id
				calendarsSynced++
			} else {
				log.Printf("Warning: failed to save calendar %s: %v", summary, err)
			}
		} else {
			gCalToLocalID[cid] = targetCal.Id
			calendarsSynced++
		}

		if !isLassoCal {
			calsToSyncEvents = append(calsToSyncEvents, cid)
		}
	}

	// Now sync events for each personal calendar
	for _, gcalID := range calsToSyncEvents {
		localCalID := gCalToLocalID[gcalID]
		if localCalID == "" {
			continue
		}

		// Find target calendar record in PocketBase to get its default color
		calRecord, _ := app.FindRecordById(proxies.CollectionCalendars, localCalID)
		defaultCalColor := "#515151"
		if calRecord != nil {
			cal := proxies.NewCalendar(calRecord)
			if cal.Color() != "" {
				defaultCalColor = cal.Color()
			}
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

			// Inbound safety check: do not import events managed/exported by Lasso
			if itm.ExtendedProperties != nil && itm.ExtendedProperties.Private != nil && itm.ExtendedProperties.Private["lasso_managed"] == "true" {
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

			formattedStart := startT.UTC().Format("2006-01-02 15:04:05.000Z")
			formattedEnd := endT.UTC().Format("2006-01-02 15:04:05.000Z")

			var targetEvt *proxies.Event
			needsEvtSave := false
			if existingEvt == nil {
				targetEvt = proxies.NewEventRecord(eventsCol)
				targetEvt.SetCalendarID(localCalID)
				targetEvt.SetGoogleEventID(itm.ID)
				targetEvt.SetDeadline(false)
				targetEvt.SetTitle(title)
				targetEvt.SetStart(formattedStart)
				targetEvt.SetEnd(formattedEnd)
				targetEvt.SetAllDay(allDay)
				targetEvt.SetColor(eventColor)
				targetEvt.SetEventLabelID(eventLabelID)
				targetEvt.SetLabelName(labelName)
				if itm.Description != "" {
					targetEvt.SetDescription(itm.Description)
				}
				needsEvtSave = true
			} else {
				targetEvt = proxies.NewEvent(existingEvt)
				if targetEvt.Title() != title {
					targetEvt.SetTitle(title)
					needsEvtSave = true
				}
				if !datesEqual(targetEvt.Start(), formattedStart) {
					targetEvt.SetStart(formattedStart)
					needsEvtSave = true
				}
				if !datesEqual(targetEvt.End(), formattedEnd) {
					targetEvt.SetEnd(formattedEnd)
					needsEvtSave = true
				}
				if targetEvt.AllDay() != allDay {
					targetEvt.SetAllDay(allDay)
					needsEvtSave = true
				}
				if targetEvt.Color() != eventColor {
					targetEvt.SetColor(eventColor)
					needsEvtSave = true
				}
				if targetEvt.EventLabelID() != eventLabelID {
					targetEvt.SetEventLabelID(eventLabelID)
					needsEvtSave = true
				}
				if targetEvt.LabelName() != labelName {
					targetEvt.SetLabelName(labelName)
					needsEvtSave = true
				}
				if itm.Description != "" && targetEvt.Description() != itm.Description {
					targetEvt.SetDescription(itm.Description)
					needsEvtSave = true
				}
			}

			if needsEvtSave {
				if err := app.Save(targetEvt); err == nil {
					if existingEvt == nil {
						eventsCreated++
					} else {
						eventsUpdated++
					}
					eventsSynced++
				} else {
					log.Printf("Warning: failed to save event %s: %v", title, err)
				}
			} else {
				eventsSynced++
			}
		}

		// Clean up deleted events in PocketBase for this calendar within the sync window
		localEvents, err := app.FindRecordsByFilter(
			proxies.CollectionEvents,
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
				evt := proxies.NewEvent(rec)
				gid := evt.GoogleEventID()
				if gid == "" {
					continue
				}
				recStart := evt.Start()
				if recStartT, _, err := parseDateString(recStart); err == nil {
					if (recStartT.After(nowUTC.AddDate(0, -1, 0)) || recStartT.Equal(nowUTC.AddDate(0, -1, 0))) &&
						(recStartT.Before(nowUTC.AddDate(0, 3, 0)) || recStartT.Equal(nowUTC.AddDate(0, 3, 0))) {
						if !activeGoogleIDs[gid] {
							if err := app.Delete(evt); err == nil {
								eventsDeleted++
							}
						}
					}
				}
			}
		}
	}

	inboundMsg := ""
	if eventsCreated == 0 && eventsUpdated == 0 && eventsDeleted == 0 {
		inboundMsg = fmt.Sprintf("Google Calendar events up to date (%d calendars, %d events evaluated).", calendarsSynced, eventsSynced)
	} else {
		inboundMsg = fmt.Sprintf("Synced %d calendars from Google Calendar (%d created, %d updated, %d removed).", calendarsSynced, eventsCreated, eventsUpdated, eventsDeleted)
	}

	return &InboundSyncResponse{
		Success:         true,
		CalendarsSynced: calendarsSynced,
		EventsSynced:    eventsSynced,
		EventsCreated:   eventsCreated,
		EventsUpdated:   eventsUpdated,
		EventsDeleted:   eventsDeleted,
		Message:         inboundMsg,
	}, nil
}
