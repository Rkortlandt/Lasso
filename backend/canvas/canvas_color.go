package canvas

import (
	"backend/proxies"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type CanvasSetColorRequest struct {
	CalendarID string `json:"calendarId"`
	CourseID   string `json:"courseId"`
	HexCode    string `json:"hexcode"`
	Color      string `json:"color"`
}

type CanvasSetColorResponse struct {
	Success    bool   `json:"success"`
	Color      string `json:"color"`
	CalendarID string `json:"calendarId,omitempty"`
	CourseID   string `json:"courseId,omitempty"`
	Message    string `json:"message,omitempty"`
}

var hexColorRegex = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func normalizeHexColor(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("color hex code cannot be empty")
	}
	if !hexColorRegex.MatchString(s) {
		return "", fmt.Errorf("invalid hex color format: %q", raw)
	}
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	return strings.ToLower(s), nil
}

// HandleCanvasSetColor updates a course's custom color both on Canvas LMS and locally in PocketBase.
func HandleCanvasSetColor(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		var req CanvasSetColorRequest
		body, err := io.ReadAll(e.Request.Body)
		if err != nil {
			return e.BadRequestError("Failed to read request body", err)
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return e.BadRequestError("Invalid JSON body", err)
		}

		rawColor := req.HexCode
		if rawColor == "" {
			rawColor = req.Color
		}
		normalizedColor, err := normalizeHexColor(rawColor)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}

		// Locate the calendar record in PocketBase
		var calendarRecord *core.Record
		if req.CalendarID != "" {
			calendarRecord, _ = app.FindFirstRecordByFilter(
				proxies.CollectionCalendars,
				"id = {:id} && user = {:user}",
				map[string]any{
					"id":   req.CalendarID,
					"user": authRecord.Id,
				},
			)
		}

		if calendarRecord == nil && req.CourseID != "" {
			calendarRecord, _ = app.FindFirstRecordByFilter(
				proxies.CollectionCalendars,
				"course_id = {:courseId} && user = {:user}",
				map[string]any{
					"courseId": req.CourseID,
					"user":     authRecord.Id,
				},
			)
		}

		courseID := req.CourseID
		if calendarRecord != nil {
			calProxy := proxies.NewCalendar(calendarRecord)
			if courseID == "" {
				courseID = calProxy.CourseID()
			}
		}

		user := proxies.NewUser(authRecord)
		canvasURL := user.CanvasURL()
		canvasToken := user.CanvasToken()

		// If connected to Canvas and courseID is present, update Canvas LMS via its API
		if canvasURL != "" && canvasToken != "" && courseID != "" {
			endpoint := fmt.Sprintf(
				"%s/api/v1/users/self/colors/course_%s",
				strings.TrimRight(canvasURL, "/"),
				courseID,
			)

			payload := map[string]string{
				"hexcode": normalizedColor,
			}
			jsonBytes, err := json.Marshal(payload)
			if err != nil {
				return e.BadRequestError("Failed to encode Canvas color payload", err)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			httpReq, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewBuffer(jsonBytes))
			if err != nil {
				return e.BadRequestError(fmt.Sprintf("Failed to construct Canvas request: %v", err), err)
			}
			httpReq.Header.Set("Authorization", "Bearer "+canvasToken)
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Accept", "application/json")

			resp, err := client.Do(httpReq)
			if err != nil {
				log.Printf("Warning: Failed to push color update to Canvas for course %s: %v", courseID, err)
			} else {
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
					respBody, _ := io.ReadAll(resp.Body)
					log.Printf("Canvas returned HTTP %d when setting color for course %s: %s", resp.StatusCode, courseID, string(respBody))
				} else {
					log.Printf("Successfully updated Canvas color for course %s to %s", courseID, normalizedColor)
				}
			}
		}

		// Update PocketBase calendar record
		calID := req.CalendarID
		if calendarRecord != nil {
			calProxy := proxies.NewCalendar(calendarRecord)
			calProxy.SetColor(normalizedColor)
			if err := app.Save(calProxy); err != nil {
				log.Printf("Failed to save calendar color locally: %v", err)
				return e.BadRequestError("Failed to persist calendar color", err)
			}
			calID = calProxy.Id

			// Also update any linked label record in the `labels` collection
			if labelID := calProxy.LabelID(); labelID != "" {
				if labelRec, err := app.FindRecordById(proxies.CollectionLabels, labelID); err == nil && labelRec != nil {
					lblProxy := proxies.NewLabel(labelRec)
					if lblProxy.Color() != normalizedColor {
						lblProxy.SetColor(normalizedColor)
						if errSaveLbl := app.Save(lblProxy); errSaveLbl != nil {
							log.Printf("Warning: failed to update linked label %s color: %v", labelID, errSaveLbl)
						}
					}
				}
			}
		}

		return e.JSON(http.StatusOK, CanvasSetColorResponse{
			Success:    true,
			Color:      normalizedColor,
			CalendarID: calID,
			CourseID:   courseID,
			Message:    "Course color updated successfully",
		})
	}
}
