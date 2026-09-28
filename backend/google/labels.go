package google

import (
	"backend/proxies"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase/core"
)

type GoogleEventLabel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	BackgroundColor string `json:"backgroundColor"`
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

// ExtractCourseTag is the exported version of extractCourseTag.
func ExtractCourseTag(courseName string, nickname string) string {
	return extractCourseTag(courseName, nickname)
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
		lastTagSync, _, err := parseDateString(lastTagSyncStr)
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
				if cal.Color() != "" && lbl.Color() != cal.Color() {
					lbl.SetColor(cal.Color())
					_ = app.Save(lbl)
				} else if lbl.Color() != "" && labelColor == "" {
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
