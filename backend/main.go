package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// API Request & Response Types
// =============================================================================

// Common
type GenericSuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// Nicknames
type SetNicknameRequest struct {
	CourseID   any    `json:"courseId,omitempty"`
	CalendarID any    `json:"calendarId,omitempty"`
	ID         any    `json:"id,omitempty"`
	Nickname   string `json:"nickname"`
}

type SetNicknameResponse struct {
	Success    bool   `json:"success"`
	CourseID   any    `json:"courseId,omitempty"`
	CalendarID any    `json:"calendarId,omitempty"`
	Nickname   string `json:"nickname"`
}

// Canvas LMS
type CanvasVerifyRequest struct {
	CanvasURL   string `json:"canvasUrl"`
	CanvasToken string `json:"canvasToken"`
}

type CanvasUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name,omitempty"`
	Email     string `json:"primary_email,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type CanvasVerifyResponse struct {
	Success     bool             `json:"success"`
	StudentName string           `json:"studentName"`
	StudentID   int              `json:"studentId"`
	AvatarURL   string           `json:"avatarUrl"`
	CourseCount int              `json:"courseCount"`
	Courses     []CanvasCourse `json:"courses"`
}

type CanvasSyncResponse struct {
	Success       bool   `json:"success"`
	CoursesSynced int    `json:"coursesSynced"`
	TasksSynced   int    `json:"tasksSynced"`
	Message       string `json:"message"`
}

type GetCanvasNicknamesResponse struct {
	Success   bool              `json:"success"`
	Nicknames map[string]string `json:"nicknames"`
}

type GetCanvasColorsResponse struct {
	CustomColors map[string]string `json:"custom_colors"`
}

// Google Calendar
type GoogleConnectRequest struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type GetGoogleNicknamesResponse struct {
	Success   bool              `json:"success"`
	Nicknames map[string]string `json:"nicknames"`
}

type GetGoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func normalizeIDString(v any) string {
	switch val := v.(type) {
	case float64:
		return fmt.Sprintf("%.0f", val)
	case float32:
		return fmt.Sprintf("%.0f", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case string:
		if strings.Contains(val, "e+") || strings.Contains(val, "E+") {
			var f float64
			if _, err := fmt.Sscanf(val, "%e", &f); err == nil {
				return fmt.Sprintf("%.0f", f)
			}
		}
		return strings.TrimSpace(val)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func refreshGoogleToken(app core.App, authRecord *core.Record) (string, error) {
	refreshToken := authRecord.GetString("google_refresh_token")
	if refreshToken == "" {
		return "", fmt.Errorf("no refresh token available")
	}

	usersCollection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return "", err
	}

	googleConfig, ok := usersCollection.OAuth2.GetProviderConfig("google")
	if !ok || googleConfig.ClientId == "" || googleConfig.ClientSecret == "" {
		return "", fmt.Errorf("google oauth provider config is incomplete")
	}

	tokenURL := googleConfig.TokenURL
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}

	data := url.Values{}
	data.Set("client_id", googleConfig.ClientId)
	data.Set("client_secret", googleConfig.ClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(tokenURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(respBytes))
	}

	var tokenResp GetGoogleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in refresh response")
	}

	authRecord.Set("google_access_token", tokenResp.AccessToken)
	authRecord.Set("google_connected", true)
	_ = app.Save(authRecord)

	return tokenResp.AccessToken, nil
}

func main() {
	app := pocketbase.New()

	// Intercept OAuth2 authentication to capture and save Google OAuth credentials
	app.OnRecordAuthWithOAuth2Request().BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		log.Printf("OAuth2 Auth event: provider=%s, email=%s, has_refresh_token=%t", e.ProviderName, e.OAuth2User.Email, e.OAuth2User.RefreshToken != "")

		if e.ProviderName == "google" && e.OAuth2User != nil {
			if e.Record != nil {
				if e.OAuth2User.AccessToken != "" {
					e.Record.Set("google_access_token", e.OAuth2User.AccessToken)
				}
				if e.OAuth2User.RefreshToken != "" {
					e.Record.Set("google_refresh_token", e.OAuth2User.RefreshToken)
				}
				if !e.OAuth2User.Expiry.IsZero() {
					e.Record.Set("google_token_expiry", e.OAuth2User.Expiry.String())
				}
				e.Record.Set("google_connected", true)
				if e.OAuth2User.Email != "" {
					e.Record.Set("google_email", e.OAuth2User.Email)
				}
			} else if e.CreateData != nil {
				if e.OAuth2User.AccessToken != "" {
					e.CreateData["google_access_token"] = e.OAuth2User.AccessToken
				}
				if e.OAuth2User.RefreshToken != "" {
					e.CreateData["google_refresh_token"] = e.OAuth2User.RefreshToken
				}
				if !e.OAuth2User.Expiry.IsZero() {
					e.CreateData["google_token_expiry"] = e.OAuth2User.Expiry.String()
				}
				e.CreateData["google_connected"] = true
				if e.OAuth2User.Email != "" {
					e.CreateData["google_email"] = e.OAuth2User.Email
				}
			}
		}

		if err := e.Next(); err != nil {
			return err
		}

		if e.ProviderName == "google" && e.Record != nil && e.OAuth2User != nil {
			if e.OAuth2User.AccessToken != "" {
				e.Record.Set("google_access_token", e.OAuth2User.AccessToken)
			}
			if e.OAuth2User.RefreshToken != "" {
				e.Record.Set("google_refresh_token", e.OAuth2User.RefreshToken)
			}
			if !e.OAuth2User.Expiry.IsZero() {
				e.Record.Set("google_token_expiry", e.OAuth2User.Expiry.String())
			}
			e.Record.Set("google_connected", true)
			if e.OAuth2User.Email != "" {
				e.Record.Set("google_email", e.OAuth2User.Email)
			}
			if err := app.Save(e.Record); err != nil {
				log.Printf("Failed to save google oauth credentials to record: %v", err)
			} else {
				log.Printf("Successfully saved Google OAuth credentials for user %s (%s)", e.Record.Id, e.OAuth2User.Email)
			}
		}

		if e.Record != nil && e.Record.Id != "" {
			_, _ = ensureUserTodoCalendar(app, e.Record.Id)
		}

		return nil
	})

	// Ensure default To Do calendar exists for users on registration and password login
	app.OnRecordCreate("users").BindFunc(func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		if e.Record != nil && e.Record.Id != "" {
			_, _ = ensureUserTodoCalendar(app, e.Record.Id)
		}
		return nil
	})

	app.OnRecordAuthWithPasswordRequest().BindFunc(func(e *core.RecordAuthWithPasswordRequestEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		if e.Record != nil && e.Record.Id != "" {
			_, _ = ensureUserTodoCalendar(app, e.Record.Id)
		}
		return nil
	})

	// Ensure an event is never marked as both an announcement and a deadline
	app.OnRecordCreate("events").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetBool("announcement") {
			e.Record.Set("deadline", false)
		} else if e.Record.GetBool("deadline") {
			e.Record.Set("announcement", false)
		}
		return e.Next()
	})
	app.OnRecordUpdate("events").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetBool("announcement") {
			e.Record.Set("deadline", false)
		} else if e.Record.GetBool("deadline") {
			e.Record.Set("announcement", false)
		}
		return e.Next()
	})

	// Register custom endpoints for Canvas LMS and Google Calendar
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Ensure grade and source_link fields exist on tasks collection
		if tasksCol, err := app.FindCollectionByNameOrId("tasks"); err == nil && tasksCol != nil {
			modified := false
			if tasksCol.Fields.GetByName("grade") == nil {
				tasksCol.Fields.Add(&core.TextField{
					Name: "grade",
				})
				modified = true
			}
			if tasksCol.Fields.GetByName("source_link") == nil {
				tasksCol.Fields.Add(&core.TextField{
					Name: "source_link",
				})
				modified = true
			}
			if modified {
				_ = app.Save(tasksCol)
			}
		}

		// Ensure Google sync and announcement fields exist on events collection and access rules match calendars/tasks
		if eventsCol, err := app.FindCollectionByNameOrId("events"); err == nil && eventsCol != nil {
			for _, fieldName := range []string{"google_event_id", "color", "description"} {
				if eventsCol.Fields.GetByName(fieldName) == nil {
					eventsCol.Fields.Add(&core.TextField{
						Name: fieldName,
					})
				}
			}
			if eventsCol.Fields.GetByName("announcement") == nil {
				eventsCol.Fields.Add(&core.BoolField{
					Name: "announcement",
				})
			}
			emptyRule := ""
			eventsCol.ListRule = &emptyRule
			eventsCol.ViewRule = &emptyRule
			eventsCol.CreateRule = &emptyRule
			eventsCol.UpdateRule = &emptyRule
			eventsCol.DeleteRule = &emptyRule
			_ = app.Save(eventsCol)
		}

		// Ensure 'todo' and 'internal' exist in calendars source select field options
		if calCol, err := app.FindCollectionByNameOrId("calendars"); err == nil && calCol != nil {
			if f := calCol.Fields.GetByName("source"); f != nil {
				if selField, ok := f.(*core.SelectField); ok {
					hasTodo := false
					hasInternal := false
					for _, val := range selField.Values {
						if val == "todo" {
							hasTodo = true
						}
						if val == "internal" {
							hasInternal = true
						}
					}
					modified := false
					if !hasTodo {
						selField.Values = append(selField.Values, "todo")
						modified = true
					}
					if !hasInternal {
						selField.Values = append(selField.Values, "internal")
						modified = true
					}
					if modified {
						if err := app.Save(calCol); err != nil {
							log.Printf("Failed to update calendars source select field: %v", err)
						} else {
							log.Printf("Successfully updated calendars source options to include 'todo' and 'internal'")
						}
					}
				}
			}
		}

		// Sanitize any existing events marked as both announcement and deadline (announcement takes precedence)
		if conflictingEvts, err := app.FindRecordsByFilter("events", "announcement = true && deadline = true", "", 500, 0); err == nil {
			for _, r := range conflictingEvts {
				r.Set("deadline", false)
				_ = app.Save(r)
			}
		}

		// Ensure default To Do calendar exists for all existing users
		if users, err := app.FindRecordsByFilter("users", "", "", 0, 0); err == nil {
			for _, u := range users {
				if _, err := ensureUserTodoCalendar(app, u.Id); err != nil {
					log.Printf("Failed to ensure To Do calendar for user %s: %v", u.Id, err)
				}
			}
		}

		se.Router.POST("/api/calendar/nickname", setNickname(app))
		se.Router.POST("/api/todo/ensure", handleEnsureTodoCalendar(app))

		// Canvas LMS endpoints
		se.Router.POST("/api/canvas/verify", handleCanvasVerify(app))
		se.Router.POST("/api/sync/canvas", canvasSync(app))
		se.Router.POST("/api/canvas/disconnect", disconnectCanvas(app))
		se.Router.GET("/api/canvas/nicknames", retriveCanvasNicknames(app))
		se.Router.POST("/api/canvas/item", getCanvasItemDetails(app))

		// Google Calendar endpoints
		se.Router.POST("/api/google/calendars", handleGoogleCalendars(app))
		se.Router.POST("/api/google/connect", handleGoogleConnect(app))
		se.Router.POST("/api/google/disconnect", disconnectGoogle(app))
		se.Router.GET("/api/google/nicknames", retriveGoogleNicknames(app))
		se.Router.POST("/api/google/lasso/ensure", handleEnsureLassoCalendar(app))
		se.Router.POST("/api/google/lasso/purge", handlePurgeLassoCalendar(app))
		se.Router.POST("/api/google/sync", syncLassoToGoogle(app))
		se.Router.POST("/api/google/sync-inbound", syncInboundGoogleCalendars(app))
		se.Router.POST("/api/google/read-events", fetchReadOnlyGoogleEvents(app))

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func getAuth(app core.App, event *core.RequestEvent) (*core.Record, error) {
	if event.Auth == nil {
		return nil, event.UnauthorizedError("Authentication required", nil)
	}

	return event.Auth, nil
}

// setNickname sets or clears a custom course or calendar nickname directly on the calendar object.
func setNickname(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		var req SetNicknameRequest
		body, err := io.ReadAll(e.Request.Body)
		if err != nil {
			return e.BadRequestError("Failed to read body", err)
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return e.BadRequestError("Invalid JSON body", err)
		}

		targetID := ""
		if req.CourseID != nil {
			targetID = normalizeIDString(req.CourseID)
		}
		if targetID == "" || targetID == "0" {
			if req.CalendarID != nil {
				targetID = fmt.Sprintf("%v", req.CalendarID)
			}
		}
		if targetID == "" || targetID == "0" {
			if req.ID != nil {
				targetID = fmt.Sprintf("%v", req.ID)
			}
		}
		if targetID == "" || targetID == "0" {
			return e.BadRequestError("courseId or calendarId is required", nil)
		}

		trimmedNickname := strings.TrimSpace(req.Nickname)
		userID := authRecord.Id

		if userID != "" {
			// 1. Find matching calendar in calendars table by course_id, calendar_id, id, or name
			cal, _ := app.FindFirstRecordByFilter(
				"calendars",
				"user = {:user} && (course_id = {:cid} || calendar_id = {:cid} || id = {:cid} || name = {:cid})",
				map[string]any{"user": userID, "cid": targetID},
			)

			if cal != nil {
				cal.Set("nickname", trimmedNickname)
				if err := app.Save(cal); err != nil {
					log.Printf("Failed to update calendar nickname on record %s: %v", cal.Id, err)
				} else {
					log.Printf("Successfully updated nickname '%s' on calendar %s (%s)", trimmedNickname, cal.Id, cal.GetString("name"))
				}
			} else {
				// If calendar doesn't have course_id yet, find by Canvas course name
				if authRecord != nil {
					canvasURL := authRecord.GetString("canvas_url")
					canvasToken := authRecord.GetString("canvas_token")
					if canvasURL != "" && canvasToken != "" {
						coursesURL := fmt.Sprintf("%s/api/v1/courses?per_page=100", canvasURL)
						client := &http.Client{Timeout: 10 * time.Second}
						if req, err := http.NewRequest("GET", coursesURL, nil); err == nil {
							req.Header.Set("Authorization", "Bearer "+canvasToken)
							req.Header.Set("Accept", "application/json")
							if resp, err := client.Do(req); err == nil && resp.StatusCode == http.StatusOK {
								var rawCourses []map[string]any
								bodyBytes, _ := io.ReadAll(resp.Body)
								resp.Body.Close()
								if json.Unmarshal(bodyBytes, &rawCourses) == nil {
									for _, c := range rawCourses {
										if normalizeIDString(c["id"]) == targetID {
											cName, _ := c["name"].(string)
											cName = strings.TrimSpace(cName)
											matchingCal, _ := app.FindFirstRecordByFilter(
												"calendars",
												"user = {:user} && name = {:name}",
												map[string]any{"user": userID, "name": cName},
											)
											if matchingCal != nil {
												matchingCal.Set("course_id", targetID)
												matchingCal.Set("nickname", trimmedNickname)
												_ = app.Save(matchingCal)
												log.Printf("Found calendar by Canvas course name %s and saved nickname '%s'", cName, trimmedNickname)
											}
											break
										}
									}
								}
							}
						}
					}
				}
			}
		}

		return e.JSON(http.StatusOK, SetNicknameResponse{
			Success:    true,
			CourseID:   req.CourseID,
			CalendarID: req.CalendarID,
			Nickname:   trimmedNickname,
		})
	}
}

func retriveCanvasNicknames(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}
		nicknames := map[string]string{}

		if authRecord != nil {
			records, err := app.FindRecordsByFilter(
				"calendars",
				"user = {:user} && nickname != ''",
				"",
				200,
				0,
				map[string]any{"user": authRecord.Id},
			)
			if err == nil {
				for _, r := range records {
					nick := r.GetString("nickname")
					if nick != "" {
						if cid := r.GetString("course_id"); cid != "" {
							nicknames[cid] = nick
						}
						nicknames[r.GetString("name")] = nick
						nicknames[r.Id] = nick
					}
				}
			}
		}

		return e.JSON(http.StatusOK, GetCanvasNicknamesResponse{
			Success:   true,
			Nicknames: nicknames,
		})
	}
}

// disconnectCanvas clears stored Canvas credentials from the user record.
func disconnectCanvas(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}
		if authRecord != nil {
			authRecord.Set("canvas_url", "")
			authRecord.Set("canvas_token", "")
			authRecord.Set("canvas_connected", false)
			authRecord.Set("canvas_student_name", "")
			_ = app.Save(authRecord)
		}
		return e.JSON(http.StatusOK, GenericSuccessResponse{
			Success: true,
		})
	}
}

// -----------------------------------------------------------------------------
// Google Calendar Handlers
// -----------------------------------------------------------------------------

// handleGoogleConnect saves Google OAuth tokens to the authenticated user record.
func handleGoogleConnect(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		var req GoogleConnectRequest
		body, _ := io.ReadAll(e.Request.Body)
		if len(body) > 0 {
			_ = json.Unmarshal(body, &req)
		}

		if req.AccessToken != "" {
			authRecord.Set("google_access_token", req.AccessToken)
		}
		if req.RefreshToken != "" {
			authRecord.Set("google_refresh_token", req.RefreshToken)
		}
		authRecord.Set("google_connected", true)
		if err := app.Save(authRecord); err != nil {
			return e.BadRequestError("Failed to save google tokens", err)
		}

		return e.JSON(http.StatusOK, GenericSuccessResponse{
			Success: true,
		})
	}
}

// retriveGoogleNicknames retrieves all saved calendar nicknames from calendar objects.
func retriveGoogleNicknames(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}
		nicknames := map[string]string{}

		if authRecord != nil {
			records, err := app.FindRecordsByFilter(
				"calendars",
				"user = {:user} && nickname != ''",
				"",
				200,
				0,
				map[string]any{"user": authRecord.Id},
			)
			if err == nil {
				for _, r := range records {
					nick := r.GetString("nickname")
					if nick != "" {
						if cid := r.GetString("calendar_id"); cid != "" {
							nicknames[cid] = nick
						}
						nicknames[r.GetString("name")] = nick
						nicknames[r.Id] = nick
					}
				}
			}
		}

		return e.JSON(http.StatusOK, GetGoogleNicknamesResponse{
			Success:   true,
			Nicknames: nicknames,
		})
	}
}

// disconnectGoogle clears stored Google Calendar credentials from the user record.
func disconnectGoogle(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}
		if authRecord != nil {
			authRecord.Set("google_access_token", "")
			authRecord.Set("google_refresh_token", "")
			authRecord.Set("google_token_expiry", "")
			authRecord.Set("google_connected", false)
			_ = app.Save(authRecord)
		}
		return e.JSON(http.StatusOK, GenericSuccessResponse{
			Success: true,
		})
	}
}

// ensureUserTodoCalendar guarantees that a user has a dedicated To Do calendar.
func ensureUserTodoCalendar(app core.App, userID string) (*core.Record, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID required")
	}

	cal, _ := app.FindFirstRecordByFilter(
		"calendars",
		"user = {:user} && (source = 'todo' || name = 'To Do')",
		map[string]any{"user": userID},
	)
	if cal != nil {
		return cal, nil
	}

	col, err := app.FindCollectionByNameOrId("calendars")
	if err != nil {
		return nil, fmt.Errorf("calendars collection not found: %w", err)
	}

	newCal := core.NewRecord(col)
	newCal.Set("user", userID)
	newCal.Set("name", "To Do")
	newCal.Set("source", "todo")
	newCal.Set("color", "#10b981")
	newCal.Set("visible", true)

	if err := app.Save(newCal); err != nil {
		return nil, fmt.Errorf("failed to create To Do calendar: %w", err)
	}

	log.Printf("Created default To Do calendar %s for user %s", newCal.Id, userID)
	return newCal, nil
}

// handleEnsureTodoCalendar handles POST /api/todo/ensure to create or fetch the user's To Do calendar.
func handleEnsureTodoCalendar(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		cal, err := ensureUserTodoCalendar(app, authRecord.Id)
		if err != nil {
			return e.BadRequestError("Failed to ensure To Do calendar", err)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"success":  true,
			"calendar": cal,
		})
	}
}

