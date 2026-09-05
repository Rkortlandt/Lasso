package main

import (
	"backend/proxies"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func getCanvasAuth(authRecord *core.Record, event *core.RequestEvent) (string, string, error) {
	if authRecord == nil {
		return "", "", event.BadRequestError("Authentication required", nil)
	}
	user := proxies.NewUser(authRecord)
	url := user.CanvasURL()
	if url == "" {
		return "", "", event.BadRequestError("No Canvas URL on Auth Record", nil)
	}
	token := user.CanvasToken()
	if token == "" {
		return "", "", event.BadRequestError("No Canvas TOKEN on Auth Record, check your canvas is connected", nil)
	}

	return url, token, nil
}

func verifyCanvasAuth(httpClient *http.Client, institutionURL string, apiToken string, event *core.RequestEvent) error {
	client := &http.Client{Timeout: 5 * time.Second}
	canvasRequest, err := http.NewRequest(http.MethodHead, fmt.Sprintf("%s/api/v1/users/self", institutionURL), nil)
	if err != nil {
		return event.BadRequestError("Invalid Canvas URL format", err)
	}
	canvasRequest.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := client.Do(canvasRequest)
	if err != nil {
		return event.BadRequestError(fmt.Sprintf("Could not connect to Canvas at %s: %v", institutionURL, err), nil)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return event.BadRequestError(fmt.Sprintf("Canvas rejected credentials (HTTP %d). Please check your token and institution URL.", resp.StatusCode), nil)
	}

	return nil
}

// handleCanvasVerify verifies credentials, fetches courses & real colors, and syncs account info.
func handleCanvasVerify(app core.App) func(event *core.RequestEvent) error {
	return func(event *core.RequestEvent) error {
		authRecord, err := getAuth(app, event)

		if err != nil {
			return err
		}

		url, token, err := getCanvasAuth(authRecord, event)

		if err != nil {
			return err
		}
		httpClient := &http.Client{Timeout: 10 * time.Second}
		err = verifyCanvasAuth(httpClient, url, token, event)
		if err != nil {
			return err
		}

		canvasUser, err := fetchCanvasUser(httpClient, url, token)
		if err != nil {
			return event.BadRequestError(fmt.Sprintf("Failed to fetch Canvas user profile: %v", err), nil)
		}

		canvasCoursesData, err := fetchCanvasCourses(httpClient, url, token)
		if err != nil {
			return err
		}

		canvasColors, err := fetchCanvasCourseColors(httpClient, url, token)
		if err != nil {
			canvasColors = map[string]string{}
		}

		// Tag each course with real color
		var courses []CanvasCourse
		for _, course := range canvasCoursesData {
			if course.AccessRestrictedByDate || course.WorkflowState == "deleted" || course.Name == "" {
				continue
			}

			courseId := course.CourseID()
			if color, ok := canvasColors["course_"+courseId]; ok && color != "" {
				course.Color = color
				course.BackgroundColor = color
			} else if color, ok := canvasColors[courseId]; ok && color != "" {
				course.Color = color
				course.BackgroundColor = color
			}

			courses = append(courses, course)
		}

		// Update user record if authenticated
		if authRecord != nil {
			user := proxies.NewUser(authRecord)
			user.SetCanvasConnected(true)
			user.SetCanvasStudentName(canvasUser.Name)
			_ = app.Save(user)

			// Also synchronize course colors into the calendars collection in PocketBase
			if errs := applyCanvasColorsToRecords(app, authRecord, canvasColors); len(errs) > 0 {
				for _, err := range errs {
					log.Printf("Warning updating Canvas course colors: %v", err)
				}
			}
		}

		return event.JSON(http.StatusOK, CanvasVerifyResponse{
			Success:     true,
			StudentName: canvasUser.Name,
			StudentID:   canvasUser.ID,
			AvatarURL:   canvasUser.AvatarURL,
			CourseCount: len(courses),
			Courses:     courses,
		})
	}
}

func applyCanvasColorsToRecords(app core.App, authRecord *core.Record, canvasColors map[string]string) []error {
	if authRecord == nil || len(canvasColors) == 0 {
		return nil
	}

	calendars := []*proxies.Calendar{}

	err := app.RecordQuery(proxies.CollectionCalendars).
		AndWhere(dbx.HashExp{"user": authRecord.Id}).
		AndWhere(dbx.NewExp("course_id != ''")).
		All(&calendars)

	if err != nil {
		return []error{fmt.Errorf("failed to query calendars for user: %w", err)}
	}

	var errs []error
	for _, calendar := range calendars {
		cid := calendar.CourseID()
		calendarColor := ""
		if col, ok := canvasColors["course_"+cid]; ok && col != "" {
			calendarColor = col
		} else if col, ok := canvasColors[cid]; ok && col != "" {
			calendarColor = col
		}

		if calendarColor != "" && calendar.Color() != calendarColor {
			calendar.SetColor(calendarColor)
			if err := app.Save(calendar); err != nil {
				errs = append(errs, fmt.Errorf("failed to update color for calendar %s: %w", calendar.Name(), err))
			}
		}
	}

	return errs
}

func executeCanvasGetRequest[T any](httpClient *http.Client, endpointURL string, apiToken string) (T, error) {
	var target T

	httpRequest, err := http.NewRequest(http.MethodGet, endpointURL, nil)
	if err != nil {
		return target, err
	}

	httpRequest.Header.Set("Authorization", "Bearer "+apiToken)
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return target, err
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return target, fmt.Errorf("canvas request returned status: %d", httpResponse.StatusCode)
	}

	if err := json.NewDecoder(httpResponse.Body).Decode(&target); err != nil {
		return target, err
	}

	return target, nil
}

type CanvasCourseTerm struct {
	ID            any        `json:"id,omitempty"`
	Name          string     `json:"name,omitempty"`
	StartAt       string     `json:"start_at,omitempty"`
	EndAt         string     `json:"end_at,omitempty"`
	ParsedStartAt *time.Time `json:"-"`
	ParsedEndAt   *time.Time `json:"-"`
}

type CanvasCourse struct {
	ID                     any               `json:"id"`
	Name                   string            `json:"name"`
	CourseCode             string            `json:"course_code,omitempty"`
	WorkflowState          string            `json:"workflow_state,omitempty"`
	AccessRestrictedByDate bool              `json:"access_restricted_by_date,omitempty"`
	EndAt                  string            `json:"end_at,omitempty"`
	ParsedEndAt            *time.Time        `json:"-"`
	IsEnded                bool              `json:"is_ended,omitempty"`
	Concluded              bool              `json:"concluded,omitempty"`
	Term                   *CanvasCourseTerm `json:"term,omitempty"`

	// Presentation / UI properties enriched during verify
	Color           string `json:"color,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
	Status          string `json:"status,omitempty"`
}

func (c *CanvasCourse) CourseID() string {
	return normalizeIDString(c.ID)
}

func normalizeCanvasCourses(courses []CanvasCourse) []CanvasCourse {
	now := time.Now()
	for i := range courses {
		c := &courses[i]

		// 1. Normalize ID (e.g. 12345.0 or 12345 -> "12345")
		c.ID = normalizeIDString(c.ID)

		// 2. Trim whitespace on text fields
		c.Name = strings.TrimSpace(c.Name)
		c.CourseCode = strings.TrimSpace(c.CourseCode)

		// 3. Lowercase and trim workflow_state (e.g. "Available" -> "available")
		c.WorkflowState = strings.ToLower(strings.TrimSpace(c.WorkflowState))

		// 4. Parse course end time and determine if ended
		c.EndAt = strings.TrimSpace(c.EndAt)
		if c.EndAt != "" {
			if t, err := time.Parse(time.RFC3339, c.EndAt); err == nil {
				c.ParsedEndAt = &t
				if t.Before(now) {
					c.IsEnded = true
				}
			}
		}

		// 5. Trim and parse term fields
		if c.Term != nil {
			c.Term.Name = strings.TrimSpace(c.Term.Name)
			c.Term.ID = normalizeIDString(c.Term.ID)

			c.Term.StartAt = strings.TrimSpace(c.Term.StartAt)
			if c.Term.StartAt != "" {
				if t, err := time.Parse(time.RFC3339, c.Term.StartAt); err == nil {
					c.Term.ParsedStartAt = &t
				}
			}

			c.Term.EndAt = strings.TrimSpace(c.Term.EndAt)
			if c.Term.EndAt != "" {
				if t, err := time.Parse(time.RFC3339, c.Term.EndAt); err == nil {
					c.Term.ParsedEndAt = &t
					if !c.IsEnded && t.Before(now) {
						c.IsEnded = true
					}
				}
			}
		}

		// 6. Pre-calculate academic status (current, previous, upcoming)
		if c.IsEnded || c.Concluded || c.WorkflowState == "completed" {
			c.Status = "previous"
		} else if c.WorkflowState == "unpublished" {
			c.Status = "upcoming"
		} else {
			c.Status = "current"
		}
	}
	return courses
}

func fetchCanvasCourses(httpClient *http.Client, institutionURL string, apiToken string) ([]CanvasCourse, error) {
	primaryEndpoint := fmt.Sprintf(
		"%s/api/v1/users/self/courses?include[]=sections&include[]=term&include[]=concluded&include[]=enrollments&state[]=available&state[]=unpublished&state[]=completed&per_page=100",
		institutionURL,
	)

	courses, primaryErr := executeCanvasGetRequest[[]CanvasCourse](httpClient, primaryEndpoint, apiToken)
	if primaryErr == nil {
		return normalizeCanvasCourses(courses), nil
	}

	fallbackEndpoint := fmt.Sprintf(
		"%s/api/v1/courses?include[]=sections&include[]=term&include[]=concluded&state[]=available&state[]=unpublished&state[]=completed&per_page=100",
		institutionURL,
	)

	fallbackCourses, fallbackErr := executeCanvasGetRequest[[]CanvasCourse](httpClient, fallbackEndpoint, apiToken)
	if fallbackErr != nil {
		return nil, fmt.Errorf("canvas course retrieval failed: primary error (%w), fallback error (%w)", primaryErr, fallbackErr)
	}

	return normalizeCanvasCourses(fallbackCourses), nil
}

func fetchCanvasCourseColors(httpClient *http.Client, institutionURL string, apiToken string) (map[string]string, error) {
	colorsURL := fmt.Sprintf("%s/api/v1/users/self/colors", institutionURL)

	payload, err := executeCanvasGetRequest[GetCanvasColorsResponse](httpClient, colorsURL, apiToken)
	if err != nil {
		return nil, err
	}

	if payload.CustomColors == nil {
		return map[string]string{}, nil
	}

	return payload.CustomColors, nil
}

func fetchCanvasUser(httpClient *http.Client, institutionURL string, apiToken string) (*CanvasUser, error) {
	userURL := fmt.Sprintf("%s/api/v1/users/self", institutionURL)
	user, err := executeCanvasGetRequest[CanvasUser](httpClient, userURL, apiToken)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func fetchCanvasCourseAssignments(httpClient *http.Client, institutionURL string, courseID string, apiToken string) ([]map[string]any, error) {
	endpointURL := fmt.Sprintf(
		"%s/api/v1/courses/%s/assignments?include[]=submission&per_page=100",
		institutionURL,
		courseID,
	)
	return executeCanvasGetRequest[[]map[string]any](httpClient, endpointURL, apiToken)
}

func fetchCanvasPlannerItems(httpClient *http.Client, institutionURL string, apiToken string, startDate string, endDate string) ([]map[string]any, error) {
	endpointURL := fmt.Sprintf(
		"%s/api/v1/users/self/planner/items?start_date=%s&end_date=%s&per_page=100",
		institutionURL,
		url.QueryEscape(startDate),
		url.QueryEscape(endDate),
	)
	return executeCanvasGetRequest[[]map[string]any](httpClient, endpointURL, apiToken)
}

func formatFloatNumber(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%.1f", f)
}

func formatGradeString(submissionData map[string]any, pointsPossible float64) string {
	if submissionData == nil {
		return ""
	}

	gradeStr := ""
	if g, ok := submissionData["entered_grade"].(string); ok && strings.TrimSpace(g) != "" {
		gradeStr = strings.TrimSpace(g)
	} else if g, ok := submissionData["grade"].(string); ok && strings.TrimSpace(g) != "" {
		gradeStr = strings.TrimSpace(g)
	}

	// If gradeStr already contains '%' or '/' (e.g. "100%" or "10/10"), return it
	if strings.Contains(gradeStr, "%") || strings.Contains(gradeStr, "/") {
		return gradeStr
	}

	var score *float64
	if s, ok := submissionData["score"].(float64); ok {
		score = &s
	} else if s, ok := submissionData["entered_score"].(float64); ok {
		score = &s
	}

	if score != nil {
		scoreVal := *score
		if pointsPossible > 0 {
			return fmt.Sprintf("%s/%s", formatFloatNumber(scoreVal), formatFloatNumber(pointsPossible))
		}
		return formatFloatNumber(scoreVal)
	}

	return gradeStr
}

func resolveCanvasAssignmentURL(assignmentData map[string]any, canvasURL string, courseID string) string {
	rawURL := ""
	if u, ok := assignmentData["html_url"].(string); ok && strings.TrimSpace(u) != "" {
		rawURL = strings.TrimSpace(u)
	} else if u, ok := assignmentData["url"].(string); ok && strings.TrimSpace(u) != "" {
		rawURL = strings.TrimSpace(u)
	}

	if rawURL != "" {
		if strings.HasPrefix(rawURL, "/") {
			return strings.TrimRight(canvasURL, "/") + rawURL
		}
		return rawURL
	}

	if idVal := assignmentData["id"]; idVal != nil && courseID != "" && canvasURL != "" {
		return fmt.Sprintf("%s/courses/%s/assignments/%v", strings.TrimRight(canvasURL, "/"), courseID, idVal)
	}

	return ""
}

func resolveCanvasPlannerItemURL(plannerItem map[string]any, plannableData map[string]any, canvasURL string, courseID string) string {
	rawURL := ""
	if u, ok := plannerItem["html_url"].(string); ok && strings.TrimSpace(u) != "" {
		rawURL = strings.TrimSpace(u)
	} else if plannableData != nil {
		if u, ok := plannableData["html_url"].(string); ok && strings.TrimSpace(u) != "" {
			rawURL = strings.TrimSpace(u)
		} else if u, ok := plannableData["url"].(string); ok && strings.TrimSpace(u) != "" {
			rawURL = strings.TrimSpace(u)
		}
	}
	if rawURL == "" {
		if u, ok := plannerItem["url"].(string); ok && strings.TrimSpace(u) != "" {
			rawURL = strings.TrimSpace(u)
		}
	}

	if rawURL != "" {
		if strings.HasPrefix(rawURL, "/") {
			return strings.TrimRight(canvasURL, "/") + rawURL
		}
		return rawURL
	}

	var plannableID any
	if plannableData != nil && plannableData["id"] != nil {
		plannableID = plannableData["id"]
	} else if plannerItem["plannable_id"] != nil {
		plannableID = plannerItem["plannable_id"]
	}

	if plannableID != nil && courseID != "" && canvasURL != "" {
		plannableType, _ := plannerItem["plannable_type"].(string)
		plannableType = strings.ToLower(strings.TrimSpace(plannableType))
		if plannableType == "" || plannableType == "assignment" {
			return fmt.Sprintf("%s/courses/%s/assignments/%v", strings.TrimRight(canvasURL, "/"), courseID, plannableID)
		} else if plannableType == "quiz" {
			return fmt.Sprintf("%s/courses/%s/quizzes/%v", strings.TrimRight(canvasURL, "/"), courseID, plannableID)
		} else if plannableType == "discussion_topic" {
			return fmt.Sprintf("%s/courses/%s/discussion_topics/%v", strings.TrimRight(canvasURL, "/"), courseID, plannableID)
		}
	}

	return ""
}

func upsertTaskRecord(
	app core.App,
	tasksCollection *core.Collection,
	authRecord *core.Record,
	calendarToUserID map[string]string,
	calendarID string,
	name string,
	dueDate string,
	status *proxies.TaskStatus,
	grade *string,
	sourceLink *string,
) error {
	if authRecord == nil || authRecord.Id == "" {
		return fmt.Errorf("valid auth record required to upsert task")
	}
	if calendarID == "" || strings.TrimSpace(name) == "" {
		return fmt.Errorf("cannot upsert: calendarID and name are required fields")
	}

	// Verify calendar exists and belongs to this user
	if calendarToUserID != nil {
		ownerID, found := calendarToUserID[calendarID]
		if !found {
			cal := &proxies.Calendar{}
			err := app.RecordQuery(proxies.CollectionCalendars).
				AndWhere(dbx.HashExp{"id": calendarID}).
				Limit(1).
				One(cal)
			if err != nil {
				return fmt.Errorf("calendar %q does not exist: %w", calendarID, err)
			}
			ownerID = cal.UserID()
			calendarToUserID[calendarID] = ownerID
		}

		if ownerID != authRecord.Id {
			return fmt.Errorf("calendar %q belongs to user %q, not %q", calendarID, ownerID, authRecord.Id)
		}
	}

	task := &proxies.Task{}

	err := app.RecordQuery(proxies.CollectionTasks).AndWhere(dbx.HashExp{
		"user": authRecord.Id,
		"name": name,
	}).Limit(1).One(task)

	if err != nil {
		// Record not found - make new one
		task = proxies.NewTaskRecord(tasksCollection)

		// Set Canvas Fields
		task.SetUserID(authRecord.Id)
		task.SetCalendarID(calendarID)
		task.SetName(name)
		task.SetDueDate(dueDate)

		if status != nil {
			task.SetStatus(*status)
		} else {
			task.SetStatus(proxies.TaskStatusTodo)
		}

		if grade != nil {
			task.SetGrade(*grade)
		}

		if sourceLink != nil {
			task.SetSourceLink(*sourceLink)
		}

		// Local Tracking
		task.SetPriority(proxies.TaskPriorityMed)

		return app.Save(task)
	}

	// Existing record — only update fields that were actually provided
	changes := false

	if task.CalendarID() != calendarID {
		task.SetCalendarID(calendarID)
		changes = true
	}
	if task.DueDate() != dueDate {
		task.SetDueDate(dueDate)
		changes = true
	}
	if status != nil && task.Status() != *status {
		task.SetStatus(*status)
		changes = true
	}
	if grade != nil && task.Grade() != *grade {
		task.SetGrade(*grade)
		changes = true
	}
	if sourceLink != nil && *sourceLink != "" && task.SourceLink() != *sourceLink {
		task.SetSourceLink(*sourceLink)
		changes = true
	}

	if !changes {
		return nil
	}

	return app.Save(task)
}

func canvasSync(app core.App) func(event *core.RequestEvent) error {
	return func(event *core.RequestEvent) error {
		startTime := time.Now()
		authRecord, err := getAuth(app, event)
		if err != nil {
			return err
		}

		// Set running status on sync_status
		_ = updateSyncStatusRunning(app, authRecord.Id, SyncOpCanvas)

		canvasURL, apiToken, err := getCanvasAuth(authRecord, event)
		if err != nil {
			_ = updateSyncStatusFinished(app, authRecord.Id, SyncOpCanvas, "error", "", err.Error(), time.Since(startTime).Milliseconds())
			return err
		}

		httpClient := &http.Client{Timeout: 10 * time.Second}
		if err := verifyCanvasAuth(httpClient, canvasURL, apiToken, event); err != nil {
			_ = updateSyncStatusFinished(app, authRecord.Id, SyncOpCanvas, "error", "", err.Error(), time.Since(startTime).Milliseconds())
			return err
		}

		rawCourses, err := fetchCanvasCourses(httpClient, canvasURL, apiToken)
		if err != nil {
			_ = updateSyncStatusFinished(app, authRecord.Id, SyncOpCanvas, "error", "", err.Error(), time.Since(startTime).Milliseconds())
			return err
		}

		canvasColors, err := fetchCanvasCourseColors(httpClient, canvasURL, apiToken)
		if err != nil {
			return err
		}

		calendarsCollection, errCal := app.FindCollectionByNameOrId(proxies.CollectionCalendars)
		if errCal != nil || calendarsCollection == nil {
			log.Printf("Error: calendars collection does not exist: %v", errCal)
			return event.InternalServerError("calendars collection does not exist", errCal)
		}

		tasksCollection, errTask := app.FindCollectionByNameOrId(proxies.CollectionTasks)
		if errTask != nil || tasksCollection == nil {
			log.Printf("Error: tasks collection does not exist: %v", errTask)
			return event.InternalServerError("tasks collection does not exist", errTask)
		}

		canvasCourseToCalendarID := make(map[string]string)
		calendarToUserID := make(map[string]string)
		syncedCourseCount := 0
		fallbackPalette := []string{"#3b82f6", "#10b981", "#8b5cf6", "#f59e0b", "#ec4899", "#06b6d4"}

		for index, courseData := range rawCourses {
			if courseData.AccessRestrictedByDate || courseData.WorkflowState == "deleted" || courseData.Name == "" {
				continue
			}

			courseName := courseData.Name
			courseID := courseData.CourseID()
			if courseID == "" {
				continue
			}

			resolvedColor := fallbackPalette[index%len(fallbackPalette)]
			if customColor, ok := canvasColors["course_"+courseID]; ok && customColor != "" {
				resolvedColor = customColor
			} else if customColor, ok := canvasColors[courseID]; ok && customColor != "" {
				resolvedColor = customColor
			}

			existingCalendarRecord, _ := app.FindFirstRecordByFilter(
				proxies.CollectionCalendars,
				"user = {:user} && (course_id = {:courseID} || name = {:name})",
				map[string]any{
					"user":     authRecord.Id,
					"courseID": courseID,
					"name":     courseName,
				},
			)

			calendarID := ""
			if existingCalendarRecord != nil {
				cal := proxies.NewCalendar(existingCalendarRecord)
				calendarID = cal.Id
				needsPersist := false
				if cal.Source() != proxies.CalendarSourceCanvas {
					cal.SetSource(proxies.CalendarSourceCanvas)
					needsPersist = true
				}
				if resolvedColor != "" && cal.Color() != resolvedColor {
					cal.SetColor(resolvedColor)
					needsPersist = true
				}
				if cal.CourseID() == "" && courseID != "" {
					cal.SetCourseID(courseID)
					needsPersist = true
				}
				if needsPersist {
					_ = app.Save(cal)
				}
			} else {
				newCal := proxies.NewCalendarRecord(calendarsCollection)
				newCal.SetUserID(authRecord.Id)
				newCal.SetName(courseName)
				newCal.SetColor(resolvedColor)
				newCal.SetSource(proxies.CalendarSourceCanvas)
				newCal.SetVisible(true)
				newCal.SetCourseID(courseID)
				if err := app.Save(newCal); err != nil {
					log.Printf("Failed to insert calendar for course %s: %v", courseName, err)
					continue
				}
				calendarID = newCal.Id
			}

			canvasCourseToCalendarID[courseID] = calendarID
			calendarToUserID[calendarID] = authRecord.Id
			syncedCourseCount++
		}

		upsertedTaskCount := 0

		for courseID, calendarID := range canvasCourseToCalendarID {
			assignments, err := fetchCanvasCourseAssignments(httpClient, canvasURL, courseID, apiToken)
			if err != nil {
				continue
			}

			for _, assignmentData := range assignments {
				assignmentTitle, _ := assignmentData["name"].(string)
				assignmentTitle = strings.TrimSpace(assignmentTitle)
				if assignmentTitle == "" {
					continue
				}

				dueAtTimestamp, _ := assignmentData["due_at"].(string)
				dueAtTimestamp = strings.TrimSpace(dueAtTimestamp)
				if dueAtTimestamp == "" {
					continue
				}

				pointsPossible := 0.0
				if pp, ok := assignmentData["points_possible"].(float64); ok {
					pointsPossible = pp
				}
				taskStatus := proxies.TaskStatusTodo
				gradeStr := ""
				if submissionData, ok := assignmentData["submission"].(map[string]any); ok && submissionData != nil {
					workflowState, _ := submissionData["workflow_state"].(string)
					workflowState = strings.ToLower(workflowState)
					submittedAtTimestamp, _ := submissionData["submitted_at"].(string)
					hasScore := submissionData["score"] != nil || submissionData["grade"] != nil
					if workflowState == "graded" || workflowState == "complete" || workflowState == "submitted" || strings.TrimSpace(submittedAtTimestamp) != "" || hasScore {
						taskStatus = proxies.TaskStatusDone
					}
					gradeStr = formatGradeString(submissionData, pointsPossible)
				}

				var gradePtr *string
				if gradeStr != "" {
					gradePtr = &gradeStr
				}
				sourceLink := resolveCanvasAssignmentURL(assignmentData, canvasURL, courseID)
				var sourceLinkPtr *string
				if sourceLink != "" {
					sourceLinkPtr = &sourceLink
				}

				if err := upsertTaskRecord(app, tasksCollection, authRecord, calendarToUserID, calendarID, assignmentTitle, dueAtTimestamp, &taskStatus, gradePtr, sourceLinkPtr); err == nil {
					upsertedTaskCount++
				}
			}
		}

		nowUTC := time.Now().UTC()
		searchStartDate := nowUTC.AddDate(0, -1, 0).Format(time.RFC3339)
		searchEndDate := nowUTC.AddDate(0, 4, 0).Format(time.RFC3339)

		plannerItems, err := fetchCanvasPlannerItems(httpClient, canvasURL, apiToken, searchStartDate, searchEndDate)
		if err != nil {
			return event.BadRequestError("Failed to fetch Canvas planner items", err)
		}

		for _, plannerItem := range plannerItems {
			plannableData, _ := plannerItem["plannable"].(map[string]any)

			dueAtTimestamp := ""
			if plannableData != nil {
				if rawDue, ok := plannableData["due_at"].(string); ok && strings.TrimSpace(rawDue) != "" {
					dueAtTimestamp = strings.TrimSpace(rawDue)
				}
			}
			if dueAtTimestamp == "" {
				if rawDue, ok := plannerItem["plannable_date"].(string); ok && strings.TrimSpace(rawDue) != "" {
					dueAtTimestamp = strings.TrimSpace(rawDue)
				}
			}
			if dueAtTimestamp == "" {
				continue
			}

			taskTitle := ""
			if plannableData != nil {
				if rawTitle, ok := plannableData["title"].(string); ok && strings.TrimSpace(rawTitle) != "" {
					taskTitle = strings.TrimSpace(rawTitle)
				} else if rawTitle, ok := plannableData["name"].(string); ok && strings.TrimSpace(rawTitle) != "" {
					taskTitle = strings.TrimSpace(rawTitle)
				}
			}
			if taskTitle == "" {
				if rawTitle, ok := plannerItem["title"].(string); ok && strings.TrimSpace(rawTitle) != "" {
					taskTitle = strings.TrimSpace(rawTitle)
				}
			}
			if taskTitle == "" {
				continue
			}

			var rawCourseIdentifier any = plannerItem["course_id"]
			if rawCourseIdentifier == nil && plannableData != nil {
				rawCourseIdentifier = plannableData["course_id"]
			}

			normalizedCourseID := normalizeIDString(rawCourseIdentifier)
			calendarID := canvasCourseToCalendarID[normalizedCourseID]

			if calendarID == "" {
				contextName, _ := plannerItem["context_name"].(string)
				contextName = strings.TrimSpace(contextName)
				if contextName != "" {
					existingCalendarRecord, _ := app.FindFirstRecordByFilter(
						proxies.CollectionCalendars,
						"user = {:user} && name = {:name}",
						map[string]any{"user": authRecord.Id, "name": contextName},
					)
					if existingCalendarRecord != nil {
						calendarID = existingCalendarRecord.Id
					} else {
						newCalendarRecord := proxies.NewCalendarRecord(calendarsCollection)
						newCalendarRecord.SetUserID(authRecord.Id)
						newCalendarRecord.SetName(contextName)
						newCalendarRecord.SetColor("#3b82f6")
						newCalendarRecord.SetSource(proxies.CalendarSourceCanvas)
						newCalendarRecord.SetVisible(true)
						if err := app.Save(newCalendarRecord); err == nil {
							calendarID = newCalendarRecord.Id
						}
					}
					if calendarID != "" && normalizedCourseID != "" {
						canvasCourseToCalendarID[normalizedCourseID] = calendarID
					}
				}
			}

			plannableType, _ := plannerItem["plannable_type"].(string)
			plannableType = strings.ToLower(strings.TrimSpace(plannableType))

			if plannableType == "announcement" {
				// Canvas announcement: Save to `events` collection as an announcement, NEVER as a task or deadline
				existingTaskRecord, _ := app.FindFirstRecordByFilter(
					proxies.CollectionTasks,
					"user = {:user} && name = {:name}",
					map[string]any{"user": authRecord.Id, "name": taskTitle},
				)
				if existingTaskRecord != nil {
					_ = app.Delete(existingTaskRecord)
				}

				eventsCollection, errEvt := app.FindCollectionByNameOrId(proxies.CollectionEvents)
				if errEvt == nil && eventsCollection != nil {
					existingAnn, _ := app.FindFirstRecordByFilter(
						proxies.CollectionEvents,
						"calendar = {:cal} && title = {:title} && (announcement = true || deadline = true)",
						map[string]any{"cal": calendarID, "title": taskTitle},
					)
					var targetAnn *proxies.Event
					if existingAnn != nil {
						targetAnn = proxies.NewEvent(existingAnn)
					} else {
						targetAnn = proxies.NewEventRecord(eventsCollection)
						targetAnn.SetCalendarID(calendarID)
					}
					targetAnn.SetTitle(taskTitle)
					targetAnn.SetStart(dueAtTimestamp)
					targetAnn.SetEnd(dueAtTimestamp)
					targetAnn.SetAllDay(false)
					targetAnn.SetAnnouncement(true)
					targetAnn.SetDeadline(false) // BE CAREFUL: Never mark as both announcement and deadline!
					if desc, ok := plannableData["message"].(string); ok && desc != "" {
						targetAnn.SetDescription(desc)
					}
					if err := app.Save(targetAnn); err != nil {
						log.Printf("Failed to save announcement event %s: %v", taskTitle, err)
					}
				}
				continue
			}

			var taskStatusPtr *proxies.TaskStatus
			var gradePtr *string

			if submissionsData, ok := plannerItem["submissions"].(map[string]any); ok && submissionsData != nil {
				isSubmitted, _ := submissionsData["submitted"].(bool)
				isGraded, _ := submissionsData["graded"].(bool)
				workflowState, _ := submissionsData["workflow_state"].(string)
				workflowState = strings.ToLower(workflowState)
				hasScore := submissionsData["score"] != nil || submissionsData["grade"] != nil
				st := proxies.TaskStatusTodo
				if isSubmitted || isGraded || workflowState == "submitted" || workflowState == "graded" || workflowState == "complete" || hasScore {
					st = proxies.TaskStatusDone
				}
				taskStatusPtr = &st

				if g := formatGradeString(submissionsData, 0); g != "" {
					gradePtr = &g
				}
			}

			sourceLink := resolveCanvasPlannerItemURL(plannerItem, plannableData, canvasURL, normalizedCourseID)
			var sourceLinkPtr *string
			if sourceLink != "" {
				sourceLinkPtr = &sourceLink
			}

			if err := upsertTaskRecord(app, tasksCollection, authRecord, calendarToUserID, calendarID, taskTitle, dueAtTimestamp, taskStatusPtr, gradePtr, sourceLinkPtr); err == nil {
				upsertedTaskCount++
			} else {
				log.Printf("Failed to upsert task %s: %v", taskTitle, err)
			}
		}

		feedbackMsg := fmt.Sprintf("Synced %d courses and %d tasks from Canvas.", syncedCourseCount, upsertedTaskCount)
		_ = updateSyncStatusFinished(
			app,
			authRecord.Id,
			SyncOpCanvas,
			"success",
			feedbackMsg,
			"",
			time.Since(startTime).Milliseconds(),
		)

		return event.JSON(http.StatusOK, CanvasSyncResponse{
			Success:       true,
			CoursesSynced: syncedCourseCount,
			TasksSynced:   upsertedTaskCount,
			Message:       feedbackMsg,
		})
	}
}
