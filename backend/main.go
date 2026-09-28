package main

import (
	"backend/google"
	"backend/proxies"
	"fmt"
	"log"
	"net/http"
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
	Success     bool           `json:"success"`
	StudentName string         `json:"studentName"`
	StudentID   int            `json:"studentId"`
	AvatarURL   string         `json:"avatarUrl"`
	CourseCount int            `json:"courseCount"`
	Courses     []CanvasCourse `json:"courses"`
}

type CanvasSyncResponse struct {
	Success       bool   `json:"success"`
	CoursesSynced int    `json:"coursesSynced"`
	TasksSynced   int    `json:"tasksSynced"`
	TasksCreated  int    `json:"tasksCreated"`
	TasksUpdated  int    `json:"tasksUpdated"`
	Message       string `json:"message"`
}

type GetCanvasColorsResponse struct {
	CustomColors map[string]string `json:"custom_colors"`
}

func main() {
	app := pocketbase.New()

	// Intercept OAuth2 authentication to capture and save Google OAuth credentials
	// AI_GEN=FALSE
	// HUMAN_REV=TRUE
	app.OnRecordAuthWithOAuth2Request().BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		log.Printf("OAuth2 Auth event: provider=%s, email=%s, has_refresh_token=%t", e.ProviderName, e.OAuth2User.Email, e.OAuth2User.RefreshToken != "")

		if e.ProviderName == "google" && e.OAuth2User != nil {
			if e.Record != nil {
				user := proxies.NewUser(e.Record)
				if e.OAuth2User.AccessToken != "" {
					user.SetGoogleAccessToken(e.OAuth2User.AccessToken)
				}
				if e.OAuth2User.RefreshToken != "" {
					user.SetGoogleRefreshToken(e.OAuth2User.RefreshToken)
				}
				if !e.OAuth2User.Expiry.IsZero() {
					user.SetGoogleTokenExpiry(e.OAuth2User.Expiry)
				}
				user.SetGoogleConnected(true)
				if e.OAuth2User.Email != "" {
					user.SetGoogleEmail(e.OAuth2User.Email)
				}
			} else if e.CreateData != nil {
				if e.OAuth2User.AccessToken != "" {
					e.CreateData["google_access_token"] = e.OAuth2User.AccessToken
				}
				if e.OAuth2User.RefreshToken != "" {
					e.CreateData["google_refresh_token"] = e.OAuth2User.RefreshToken
				}
				if !e.OAuth2User.Expiry.IsZero() {
					e.CreateData["google_token_expiry"] = e.OAuth2User.Expiry
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
			user := proxies.NewUser(e.Record)
			if e.OAuth2User.AccessToken != "" {
				user.SetGoogleAccessToken(e.OAuth2User.AccessToken)
			}
			if e.OAuth2User.RefreshToken != "" {
				user.SetGoogleRefreshToken(e.OAuth2User.RefreshToken)
			}
			if !e.OAuth2User.Expiry.IsZero() {
				user.SetGoogleTokenExpiry(e.OAuth2User.Expiry)
			}
			user.SetGoogleConnected(true)
			if e.OAuth2User.Email != "" {
				user.SetGoogleEmail(e.OAuth2User.Email)
			}
			if err := app.Save(user); err != nil {
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

	// Ensure To Do calendar exists when a task with source "todo" is created
	app.OnRecordCreate(proxies.CollectionTasks).BindFunc(func(e *core.RecordEvent) error {
		task := proxies.NewTask(e.Record)
		userID := task.UserID()
		if userID != "" && task.Source() == proxies.TaskSourceTodo {
			todoCal, err := ensureUserTodoCalendar(app, userID)
			if err == nil && todoCal != nil && task.CalendarID() == "" {
				task.SetCalendarID(todoCal.Id)
			}
		}

		return e.Next()
	})

	// Ensure an event is never marked as both an announcement and a deadline
	app.OnRecordCreate(proxies.CollectionEvents).BindFunc(func(e *core.RecordEvent) error {
		evt := proxies.NewEvent(e.Record)
		if evt.Announcement() {
			evt.SetDeadline(false)
		} else if evt.Deadline() {
			evt.SetAnnouncement(false)
		}
		return e.Next()
	})
	app.OnRecordUpdate(proxies.CollectionEvents).BindFunc(func(e *core.RecordEvent) error {
		evt := proxies.NewEvent(e.Record)
		if evt.Announcement() {
			evt.SetDeadline(false)
		} else if evt.Deadline() {
			evt.SetAnnouncement(false)
		}
		return e.Next()
	})

	// Flag matching google_sync_registry rows as pending_delete when a task is deleted
	app.OnRecordDelete("tasks").BindFunc(func(e *core.RecordEvent) error {
		if e.Record != nil && e.Record.Id != "" {
			entries, err := app.FindRecordsByFilter(
				proxies.CollectionSyncRegistry,
				"entity_type = 'task' && entity_id = {:id} && status = 'active'",
				"",
				0,
				0,
				map[string]any{"id": e.Record.Id},
			)
			if err == nil {
				for _, rec := range entries {
					reg := proxies.NewSyncRegistry(rec)
					reg.SetStatus(proxies.SyncRegistryStatusPendingDelete)
					if err := app.Save(reg); err != nil {
						log.Printf("Warning: failed to mark task %s as pending_delete in sync registry: %v", e.Record.Id, err)
					}
				}
			}
		}
		return e.Next()
	})

	// Flag matching google_sync_registry rows as pending_delete when an event is deleted
	app.OnRecordDelete("events").BindFunc(func(e *core.RecordEvent) error {
		if e.Record != nil && e.Record.Id != "" {
			entries, err := app.FindRecordsByFilter(
				proxies.CollectionSyncRegistry,
				"entity_type = 'event' && entity_id = {:id} && status = 'active'",
				"",
				0,
				0,
				map[string]any{"id": e.Record.Id},
			)
			if err == nil {
				for _, rec := range entries {
					reg := proxies.NewSyncRegistry(rec)
					reg.SetStatus(proxies.SyncRegistryStatusPendingDelete)
					if err := app.Save(reg); err != nil {
						log.Printf("Warning: failed to mark event %s as pending_delete in sync registry: %v", e.Record.Id, err)
					}
				}
			}
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

		// Ensure 'todo' and 'internal' exist in calendars source select field options,
		// and ensure 'start_date' and 'end_date' text fields exist.
		if calCol, err := app.FindCollectionByNameOrId("calendars"); err == nil && calCol != nil {
			modified := false
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
					if !hasTodo {
						selField.Values = append(selField.Values, "todo")
						modified = true
					}
					if !hasInternal {
						selField.Values = append(selField.Values, "internal")
						modified = true
					}
				}
			}
			for _, dateFieldName := range []string{"start_date", "end_date"} {
				if calCol.Fields.GetByName(dateFieldName) == nil {
					calCol.Fields.Add(&core.DateField{
						Name: dateFieldName,
					})
					modified = true
				}
			}
			if calCol.Fields.GetByName("google_label_id") == nil {
				calCol.Fields.Add(&core.TextField{
					Name: "google_label_id",
				})
				modified = true
			}
			if modified {
				if err := app.Save(calCol); err != nil {
					log.Printf("Failed to update calendars collection schema: %v", err)
				} else {
					log.Printf("Successfully updated calendars collection schema")
				}
			}
		}

		// Ensure google_tags_synced_at and google_export_calendar_id exist on sync_status collection
		if syncCol, err := app.FindCollectionByNameOrId(proxies.CollectionSyncStatus); err == nil && syncCol != nil {
			modified := false
			if syncCol.Fields.GetByName("google_tags_synced_at") == nil {
				syncCol.Fields.Add(&core.DateField{
					Name: "google_tags_synced_at",
				})
				modified = true
			}
			if syncCol.Fields.GetByName("google_export_calendar_id") == nil {
				syncCol.Fields.Add(&core.TextField{
					Name: "google_export_calendar_id",
				})
				modified = true
			}
			if modified {
				_ = app.Save(syncCol)
			}
		}

		// Ensure google_sync_registry collection exists and has necessary indexes
		if regCol, err := app.FindCollectionByNameOrId(proxies.CollectionSyncRegistry); err != nil || regCol == nil {
			usersCol, _ := app.FindCollectionByNameOrId("users")
			col := core.NewCollection(proxies.CollectionSyncRegistry, proxies.CollectionSyncRegistry)
			if usersCol != nil {
				col.Fields.Add(&core.RelationField{
					Name:          "user",
					CollectionId:  usersCol.Id,
					CascadeDelete: true,
					Required:      true,
				})
			}
			col.Fields.Add(&core.TextField{Name: "entity_id", Required: true})
			col.Fields.Add(&core.SelectField{
				Name:     "entity_type",
				Values:   []string{"task", "event"},
				Required: true,
			})
			col.Fields.Add(&core.TextField{Name: "google_calendar_id", Required: true})
			col.Fields.Add(&core.TextField{Name: "google_event_id", Required: true})
			col.Fields.Add(&core.DateField{Name: "synced_at", Required: true})
			col.Fields.Add(&core.SelectField{
				Name:     "status",
				Values:   []string{"pending_delete", "deleted", "active"},
				Required: true,
			})
			col.AddIndex("idx_sync_reg_unique", true, "`user`, `google_calendar_id`, `entity_type`, `entity_id`", "")
			col.AddIndex("idx_sync_reg_status", false, "`user`, `status`", "")
			emptyRule := "@request.auth.id = user.id"
			col.ListRule = &emptyRule
			col.ViewRule = &emptyRule
			_ = app.Save(col)
		} else {
			colModified := false
			if regCol.GetIndex("idx_sync_reg_unique") == "" {
				regCol.AddIndex("idx_sync_reg_unique", true, "`user`, `google_calendar_id`, `entity_type`, `entity_id`", "")
				colModified = true
			}
			if regCol.GetIndex("idx_sync_reg_status") == "" {
				regCol.AddIndex("idx_sync_reg_status", false, "`user`, `status`", "")
				colModified = true
			}
			if colModified {
				_ = app.Save(regCol)
			}
		}

		// Sanitize any existing events marked as both announcement and deadline (announcement takes precedence)
		if conflictingEvts, err := app.FindRecordsByFilter(proxies.CollectionEvents, "announcement = true && deadline = true", "", 500, 0); err == nil {
			for _, r := range conflictingEvts {
				evt := proxies.NewEvent(r)
				evt.SetDeadline(false)
				_ = app.Save(evt)
			}
		}

		// Ensure default To Do calendar exists for all existing users
		if users, err := app.FindRecordsByFilter(proxies.CollectionUsers, "", "", 0, 0); err == nil {
			for _, u := range users {
				if _, err := ensureUserTodoCalendar(app, u.Id); err != nil {
					log.Printf("Failed to ensure To Do calendar for user %s: %v", u.Id, err)
				}
			}
		}

		// Automatically recover and re-run any syncs that were interrupted by an abrupt shutdown or crash
		go func() {
			time.Sleep(1 * time.Second)
			recoverInterruptedSyncs(app)
		}()

		// Canvas LMS endpoints
		se.Router.POST("/api/canvas/verify", handleCanvasVerify(app))
		se.Router.POST("/api/sync/canvas", handleCanvasSync(app))
		se.Router.POST("/api/canvas/disconnect", handleCanvasDisconnect(app))
		se.Router.POST("/api/canvas/item", handleCanvasItemDetails(app))
		se.Router.POST("/api/canvas/color", handleCanvasSetColor(app))
		se.Router.PUT("/api/canvas/color", handleCanvasSetColor(app))

		// Google Calendar endpoints
		se.Router.POST("/api/google/disconnect", handleGoogleDisconnect(app))
		se.Router.POST("/api/google/lasso/purge", google.HandlePurgeLassoCalendar(app))
		se.Router.POST("/api/google/sync", google.HandleSyncLassoToGoogle(app))
		se.Router.POST("/api/google/sync-inbound", google.HandleSyncInboundGoogle(app))

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

// handleCanvasDisconnect clears stored Canvas credentials from the user record.
func handleCanvasDisconnect(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}
		if authRecord != nil {
			user := proxies.NewUser(authRecord)
			user.SetCanvasURL("")
			user.SetCanvasToken("")
			user.SetCanvasConnected(false)
			user.SetCanvasStudentName("")
			_ = app.Save(user)
		}
		return e.JSON(http.StatusOK, GenericSuccessResponse{
			Success: true,
		})
	}
}

// -----------------------------------------------------------------------------
// Google Calendar Handlers
// -----------------------------------------------------------------------------

// handleGoogleDisconnect clears stored Google Calendar credentials from the user record.
func handleGoogleDisconnect(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}
		if authRecord != nil {
			user := proxies.NewUser(authRecord)
			user.SetGoogleAccessToken("")
			user.SetGoogleRefreshToken("")
			user.SetGoogleTokenExpiry("")
			user.SetGoogleConnected(false)
			_ = app.Save(user)
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

	col, err := app.FindCollectionByNameOrId(proxies.CollectionCalendars)
	if err != nil {
		return nil, fmt.Errorf("calendars collection not found: %w", err)
	}

	newCal := proxies.NewCalendarRecord(col)
	newCal.SetUserID(userID)
	newCal.SetName("To Do")
	newCal.SetSource(proxies.CalendarSourceTodo)
	newCal.SetColor("#10b981")
	newCal.SetVisible(true)

	if err := app.Save(newCal); err != nil {
		return nil, fmt.Errorf("failed to create To Do calendar: %w", err)
	}

	log.Printf("Created default To Do calendar %s for user %s", newCal.Id, userID)
	return newCal.ProxyRecord(), nil
}
