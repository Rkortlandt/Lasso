package google

import (
	"backend/proxies"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

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
	Recurrence         []string                       `json:"recurrence,omitempty"`
	RecurringEventID   string                         `json:"recurringEventId,omitempty"`
	OriginalStartTime  *GoogleDateOrDateTime          `json:"originalStartTime,omitempty"`
	ExtendedProperties *GoogleEventExtendedProperties `json:"extendedProperties"`
}

type InboundSyncResponse struct {
	Success         bool   `json:"success"`
	CalendarsSynced int    `json:"calendarsSynced"`
	EventsSynced    int    `json:"eventsSynced"`
	EventsCreated   int    `json:"eventsCreated"`
	EventsUpdated   int    `json:"eventsUpdated"`
	EventsDeleted   int    `json:"eventsDeleted"`
	Message         string `json:"message"`
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

// ParseDateString is the exported version of parseDateString.
func ParseDateString(dateStr string) (time.Time, bool, error) {
	return parseDateString(dateStr)
}

// datesEqual checks whether two date/time strings represent the exact same timestamp.
func datesEqual(d1, d2 string) bool {
	t1 := strings.TrimSpace(d1)
	t2 := strings.TrimSpace(d2)
	if t1 == t2 {
		return true
	}
	if t1 == "" || t2 == "" {
		return false
	}
	time1, _, err1 := parseDateString(t1)
	time2, _, err2 := parseDateString(t2)
	if err1 == nil && err2 == nil {
		return time1.Equal(time2)
	}
	return false
}

// DatesEqual is the exported version of datesEqual.
func DatesEqual(d1, d2 string) bool {
	return datesEqual(d1, d2)
}

// handleSyncInboundGoogle handles the POST /api/google/sync-inbound endpoint
func handleSyncInboundGoogle(app core.App) func(e *core.RequestEvent) error {
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

	calendarsCol, err := app.FindCollectionByNameOrId(proxies.CollectionCalendars)
	if err != nil {
		return nil, fmt.Errorf("calendars collection not found: %w", err)
	}
	eventsCol, err := app.FindCollectionByNameOrId(proxies.CollectionEvents)
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
	timeMin := nowUTC.AddDate(0, -6, 0).Format(time.RFC3339)
	timeMax := nowUTC.AddDate(0, 6, 0).Format(time.RFC3339)

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
			proxies.CollectionCalendars,
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

		baseURL := fmt.Sprintf(
			"https://www.googleapis.com/calendar/v3/calendars/%s/events?singleEvents=false&timeMin=%s&timeMax=%s&maxResults=250&eventLabelVersion=1",
			url.PathEscape(gcalID),
			url.QueryEscape(timeMin),
			url.QueryEscape(timeMax),
		)

		var allItems []GoogleEventItem
		nextPageToken := ""
		for {
			apiURL := baseURL
			if nextPageToken != "" {
				apiURL += "&pageToken=" + url.QueryEscape(nextPageToken)
			}

			evResp, _, err := makeGoogleAPIRequest(app, authRecord, "GET", apiURL, nil)
			if err != nil {
				log.Printf("Warning: failed to fetch events for cal %s: %v", gcalID, err)
				break
			}
			if evResp.StatusCode != http.StatusOK {
				evResp.Body.Close()
				break
			}

			var listResp GoogleEventListResponse
			if err := json.NewDecoder(evResp.Body).Decode(&listResp); err != nil {
				evResp.Body.Close()
				break
			}
			evResp.Body.Close()

			allItems = append(allItems, listResp.Items...)
			if listResp.NextPageToken == "" {
				break
			}
			nextPageToken = listResp.NextPageToken
		}

		activeGoogleIDs := make(map[string]bool)
		googleIDToPbID := make(map[string]string)
		googleIDToMasterEvt := make(map[string]*proxies.Event)

		// ---------------------------------------------------------------------
		// PASS 1: Master recurring events & Standalone non-recurring events
		// ---------------------------------------------------------------------
		for _, itm := range allItems {
			if itm.Status == "cancelled" || itm.RecurringEventID != "" {
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

			// Extract recurrence rule if present
			rruleStr := ""
			if len(itm.Recurrence) > 0 {
				for _, r := range itm.Recurrence {
					if strings.HasPrefix(r, "RRULE:") || strings.HasPrefix(r, "FREQ=") {
						rruleStr = r
						break
					}
				}
			}

			activeGoogleIDs[itm.ID] = true

			// Look for existing event in PocketBase
			existingEvt, _ := app.FindFirstRecordByFilter(
				proxies.CollectionEvents,
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
				targetEvt.SetRecurr(rruleStr)
				targetEvt.SetRecurrEventID("")
				targetEvt.SetRecurrOgDate("")
				targetEvt.SetSource("google")
				if itm.Start.TimeZone != "" {
					targetEvt.SetTimezone(itm.Start.TimeZone)
				}
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
				if targetEvt.Recurr() != rruleStr {
					targetEvt.SetRecurr(rruleStr)
					needsEvtSave = true
				}
				if targetEvt.RecurrEventID() != "" {
					targetEvt.SetRecurrEventID("")
					needsEvtSave = true
				}
				if targetEvt.RecurrOgDate() != "" {
					targetEvt.SetRecurrOgDate("")
					needsEvtSave = true
				}
				if targetEvt.Source() != "google" {
					targetEvt.SetSource("google")
					needsEvtSave = true
				}
				if itm.Start.TimeZone != "" && targetEvt.Timezone() != itm.Start.TimeZone {
					targetEvt.SetTimezone(itm.Start.TimeZone)
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

			googleIDToPbID[itm.ID] = targetEvt.Id
			googleIDToMasterEvt[itm.ID] = targetEvt
		}

		// ---------------------------------------------------------------------
		// PASS 2: Exceptions (Modified occurrences) & Cancellations (EXDATE)
		// ---------------------------------------------------------------------
		for _, itm := range allItems {
			if itm.RecurringEventID == "" && itm.Status != "cancelled" {
				continue
			}

			// Inbound safety check: do not import events managed/exported by Lasso
			if itm.ExtendedProperties != nil && itm.ExtendedProperties.Private != nil && itm.ExtendedProperties.Private["lasso_managed"] == "true" {
				continue
			}

			// Find master event
			var masterEvt *proxies.Event
			if itm.RecurringEventID != "" {
				masterEvt = googleIDToMasterEvt[itm.RecurringEventID]
				if masterEvt == nil {
					if masterRec, _ := app.FindFirstRecordByFilter(
						proxies.CollectionEvents,
						"calendar = {:calId} && google_event_id = {:gid}",
						map[string]any{"calId": localCalID, "gid": itm.RecurringEventID},
					); masterRec != nil {
						masterEvt = proxies.NewEvent(masterRec)
						googleIDToMasterEvt[itm.RecurringEventID] = masterEvt
						googleIDToPbID[itm.RecurringEventID] = masterEvt.Id
					}
				}
			}

			// Case A: Cancelled occurrence -> Add originalStartTime to master's exdate
			if itm.Status == "cancelled" {
				if masterEvt != nil && itm.OriginalStartTime != nil {
					origStartVal := itm.OriginalStartTime.DateTime
					if origStartVal == "" {
						origStartVal = itm.OriginalStartTime.Date
					}
					if origT, _, errOrig := parseDateString(origStartVal); errOrig == nil {
						formattedOrig := origT.UTC().Format("2006-01-02 15:04:05.000Z")

						var exdates []string
						if raw := masterEvt.Exdate(); raw != nil {
							if arr, ok := raw.([]any); ok {
								for _, item := range arr {
									if s, ok := item.(string); ok {
										exdates = append(exdates, s)
									}
								}
							} else if strArr, ok := raw.([]string); ok {
								exdates = strArr
							}
						}

						alreadyExists := false
						for _, ex := range exdates {
							if datesEqual(ex, formattedOrig) {
								alreadyExists = true
								break
							}
						}

						if !alreadyExists {
							exdates = append(exdates, formattedOrig)
							masterEvt.SetExdate(exdates)
							if err := app.Save(masterEvt); err == nil {
								eventsUpdated++
							}
						}
					}
				}
				continue
			}

			// Case B: Modified instance exception (active occurrence with changes)
			if itm.RecurringEventID != "" {
				activeGoogleIDs[itm.ID] = true

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
					if masterEvt != nil && masterEvt.Title() != "" {
						title = masterEvt.Title()
					} else {
						title = "(No title)"
					}
				}

				// Parse original start time
				formattedOrig := ""
				if itm.OriginalStartTime != nil {
					origStartVal := itm.OriginalStartTime.DateTime
					if origStartVal == "" {
						origStartVal = itm.OriginalStartTime.Date
					}
					if origT, _, errOrig := parseDateString(origStartVal); errOrig == nil {
						formattedOrig = origT.UTC().Format("2006-01-02 15:04:05.000Z")
					}
				}

				masterPbID := ""
				if masterEvt != nil {
					masterPbID = masterEvt.Id
				}

				existingEvt, _ := app.FindFirstRecordByFilter(
					proxies.CollectionEvents,
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
					targetEvt.SetRecurr("")
					targetEvt.SetRecurrEventID(masterPbID)
					targetEvt.SetRecurrOgDate(formattedOrig)
					targetEvt.SetSource("google")
					if itm.Start.TimeZone != "" {
						targetEvt.SetTimezone(itm.Start.TimeZone)
					}
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
					if targetEvt.Recurr() != "" {
						targetEvt.SetRecurr("")
						needsEvtSave = true
					}
					if targetEvt.RecurrEventID() != masterPbID {
						targetEvt.SetRecurrEventID(masterPbID)
						needsEvtSave = true
					}
					if !datesEqual(targetEvt.RecurrOgDate(), formattedOrig) {
						targetEvt.SetRecurrOgDate(formattedOrig)
						needsEvtSave = true
					}
					if targetEvt.Source() != "google" {
						targetEvt.SetSource("google")
						needsEvtSave = true
					}
					if itm.Start.TimeZone != "" && targetEvt.Timezone() != itm.Start.TimeZone {
						targetEvt.SetTimezone(itm.Start.TimeZone)
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
						log.Printf("Warning: failed to save exception event %s: %v", title, err)
					}
				} else {
					eventsSynced++
				}
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
					if (recStartT.After(nowUTC.AddDate(0, -6, 0)) || recStartT.Equal(nowUTC.AddDate(0, -6, 0))) &&
						(recStartT.Before(nowUTC.AddDate(0, 6, 0)) || recStartT.Equal(nowUTC.AddDate(0, 6, 0))) {
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

func HandleSyncInboundGoogle(app core.App) func(e *core.RequestEvent) error {
	return handleSyncInboundGoogle(app)
}

func RunInboundGoogleSync(app core.App, authRecord *core.Record) (*InboundSyncResponse, error) {
	return runInboundGoogleSync(app, authRecord)
}
