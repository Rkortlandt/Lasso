package main

import (
	"encoding/json"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func getCanvasAuth(authRecord *core.Record, event *core.RequestEvent) (string, string, error) {
	url := authRecord.GetString("canvas_url")
	if url == "" {
		return "", "", event.BadRequestError("No Canvas URL on Auth Record", nil)
	}
	token := authRecord.GetString("canvas_token")
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
						if cid := r.GetString("course_id"); cid != "" {
							localNicknames[cid] = nick
							localNicknames[normalizeIDString(cid)] = nick
						}
						localNicknames[r.GetString("name")] = nick
						localNicknames[r.Id] = nick
					}
				}
			}
		}

		rawCourses, err := fetchCanvasCourses(httpClient, url, token)
		if err != nil {
			return err
		}

		canvasColors, err := fetchCanvasCourseColors(httpClient, url, token)
		if err != nil {
			canvasColors = map[string]string{}
		}

		// Clean and tag each course with accurate status, real color, and local nicknames
		var courses []map[string]any
		for _, rc := range rawCourses {
			if restricted, ok := rc["access_restricted_by_date"].(bool); ok && restricted {
				continue
			}
			name, _ := rc["name"].(string)
			if strings.TrimSpace(name) == "" {
				continue
			}

			wfState := strings.ToLower(fmt.Sprintf("%v", rc["workflow_state"]))
			if wfState == "deleted" {
				continue
			}

			now := time.Now()
			isEnded := false
			if endAtStr, ok := rc["end_at"].(string); ok && strings.TrimSpace(endAtStr) != "" {
				if t, err := time.Parse(time.RFC3339, endAtStr); err == nil && t.Before(now) {
					isEnded = true
				}
			}
			if !isEnded {
				if termMap, ok := rc["term"].(map[string]any); ok {
					if termEnd, ok := termMap["end_at"].(string); ok && strings.TrimSpace(termEnd) != "" {
						if t, err := time.Parse(time.RFC3339, termEnd); err == nil && t.Before(now) {
							isEnded = true
						}
					}
				}
			}

			concluded, _ := rc["concluded"].(bool)

			status := "current"
			if isEnded || concluded || wfState == "completed" {
				status = "previous"
			} else if wfState == "unpublished" {
				status = "upcoming"
			} else {
				status = "current"
			}

			cid := normalizeIDString(rc["id"])
			if color, ok := canvasColors["course_"+cid]; ok && color != "" {
				rc["color"] = color
				rc["backgroundColor"] = color
			} else if color, ok := canvasColors[cid]; ok && color != "" {
				rc["color"] = color
				rc["backgroundColor"] = color
			}

			origName, _ := rc["name"].(string)
			if nick, ok := localNicknames[cid]; ok && nick != "" {
				rc["nickname"] = nick
				rc["original_name"] = origName
				rc["name"] = nick
			} else if nick, ok := localNicknames[origName]; ok && nick != "" {
				rc["nickname"] = nick
				rc["original_name"] = origName
				rc["name"] = nick
			}

			rc["status"] = status
			courses = append(courses, rc)
		}

		// Update user record if authenticated
		if authRecord != nil {
			authRecord.Set("canvas_connected", true)
			authRecord.Set("canvas_student_name", canvasUser.Name)
			_ = app.Save(authRecord)

			// Also synchronize course colors into the calendars collection in PocketBase
			if len(canvasColors) > 0 {
				if calRecords, errCal := app.FindRecordsByFilter(
					"calendars",
					"user = {:user} && source = 'canvas'",
					"",
					200,
					0,
					map[string]any{"user": authRecord.Id},
				); errCal == nil {
					for _, cal := range calRecords {
						cid := cal.GetString("course_id")
						cColor := ""
						if color, ok := canvasColors["course_"+cid]; ok && color != "" {
							cColor = color
						} else if color, ok := canvasColors[cid]; ok && color != "" {
							cColor = color
						}
						if cColor != "" && cal.GetString("color") != cColor {
							cal.Set("color", cColor)
							_ = app.Save(cal)
						}
					}
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

func fetchCanvasCourses(httpClient *http.Client, institutionURL string, apiToken string) ([]map[string]any, error) {
	primaryEndpoint := fmt.Sprintf(
		"%s/api/v1/users/self/courses?include[]=sections&include[]=term&include[]=concluded&include[]=enrollments&state[]=available&state[]=unpublished&state[]=completed&per_page=100",
		institutionURL,
	)

	courses, primaryErr := executeCanvasGetRequest[[]map[string]any](httpClient, primaryEndpoint, apiToken)
	if primaryErr == nil {
		return courses, nil
	}

	fallbackEndpoint := fmt.Sprintf(
		"%s/api/v1/courses?include[]=sections&include[]=term&include[]=concluded&state[]=available&state[]=unpublished&state[]=completed&per_page=100",
		institutionURL,
	)

	fallbackCourses, fallbackErr := executeCanvasGetRequest[[]map[string]any](httpClient, fallbackEndpoint, apiToken)
	if fallbackErr != nil {
		return nil, fmt.Errorf("canvas course retrieval failed: primary error (%w), fallback error (%w)", primaryErr, fallbackErr)
	}

	return fallbackCourses, nil
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

func upsertTaskRecord(app core.App, tasksCollection *core.Collection, userID string, calendarID string, taskName string, dueDate string, status string, grade string) error {
	existingTaskRecord, _ := app.FindFirstRecordByFilter(
		"tasks",
		"user = {:user} && name = {:name}",
		map[string]any{
			"user": userID,
			"name": taskName,
		},
	)

	targetRecord := existingTaskRecord
	if targetRecord == nil {
		targetRecord = core.NewRecord(tasksCollection)
		targetRecord.Set("user", userID)
		targetRecord.Set("name", taskName)
		targetRecord.Set("priority", "med")
	}

	targetRecord.Set("calendar", calendarID)
	if tasksCollection.Fields.GetByName("calendar_id") != nil {
		targetRecord.Set("calendar_id", calendarID)
	}
	targetRecord.Set("due_date", dueDate)

	if grade != "" {
		targetRecord.Set("grade", grade)
	}

	// If the user or a previous sync already marked this task as done, preserve it
	if existingTaskRecord != nil && existingTaskRecord.GetString("status") == "done" {
		targetRecord.Set("status", "done")
	} else {
		targetRecord.Set("status", status)
	}

	return app.Save(targetRecord)
}

func canvasSync(app core.App) func(event *core.RequestEvent) error {
	return func(event *core.RequestEvent) error {
		authRecord, err := getAuth(app, event)
		if err != nil {
			return err
		}

		canvasURL, apiToken, err := getCanvasAuth(authRecord, event)
		if err != nil {
			return err
		}

		httpClient := &http.Client{Timeout: 10 * time.Second}
		if err := verifyCanvasAuth(httpClient, canvasURL, apiToken, event); err != nil {
			return err
		}

		rawCourses, err := fetchCanvasCourses(httpClient, canvasURL, apiToken)
		if err != nil {
			return err
		}

		canvasColors, err := fetchCanvasCourseColors(httpClient, canvasURL, apiToken)
		if err != nil {
			return err
		}

		calendarsCollection, errCal := app.FindCollectionByNameOrId("calendars")
		if errCal != nil || calendarsCollection == nil {
			log.Printf("Error: calendars collection does not exist: %v", errCal)
			return event.InternalServerError("calendars collection does not exist", errCal)
		}

		tasksCollection, errTask := app.FindCollectionByNameOrId("tasks")
		if errTask != nil || tasksCollection == nil {
			log.Printf("Error: tasks collection does not exist: %v", errTask)
			return event.InternalServerError("tasks collection does not exist", errTask)
		}

		canvasCourseToCalendarID := make(map[string]string)
		syncedCourseCount := 0
		fallbackPalette := []string{"#3b82f6", "#10b981", "#8b5cf6", "#f59e0b", "#ec4899", "#06b6d4"}

		for index, courseData := range rawCourses {
			if restricted, ok := courseData["access_restricted_by_date"].(bool); ok && restricted {
				continue
			}
			workflowState := strings.ToLower(fmt.Sprintf("%v", courseData["workflow_state"]))
			if workflowState == "deleted" {
				continue
			}
			courseName, _ := courseData["name"].(string)
			courseName = strings.TrimSpace(courseName)
			if courseName == "" {
				continue
			}

			courseID := normalizeIDString(courseData["id"])
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
				"calendars",
				"user = {:user} && (course_id = {:courseID} || name = {:name})",
				map[string]any{
					"user":     authRecord.Id,
					"courseID": courseID,
					"name":     courseName,
				},
			)

			calendarID := ""
			if existingCalendarRecord != nil {
				calendarID = existingCalendarRecord.Id
				needsPersist := false
				if existingCalendarRecord.GetString("source") != "canvas" {
					existingCalendarRecord.Set("source", "canvas")
					needsPersist = true
				}
				if resolvedColor != "" && existingCalendarRecord.GetString("color") != resolvedColor {
					existingCalendarRecord.Set("color", resolvedColor)
					needsPersist = true
				}
				if existingCalendarRecord.GetString("course_id") == "" && courseID != "" {
					existingCalendarRecord.Set("course_id", courseID)
					needsPersist = true
				}
				if needsPersist {
					_ = app.Save(existingCalendarRecord)
				}
			} else {
				newCalendarRecord := core.NewRecord(calendarsCollection)
				newCalendarRecord.Set("user", authRecord.Id)
				newCalendarRecord.Set("name", courseName)
				newCalendarRecord.Set("color", resolvedColor)
				newCalendarRecord.Set("source", "canvas")
				newCalendarRecord.Set("visible", true)
				newCalendarRecord.Set("course_id", courseID)
				if err := app.Save(newCalendarRecord); err != nil {
					log.Printf("Failed to insert calendar for course %s: %v", courseName, err)
					continue
				}
				calendarID = newCalendarRecord.Id
			}

			canvasCourseToCalendarID[courseID] = calendarID
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
				taskStatus := "todo"
				gradeStr := ""
				if submissionData, ok := assignmentData["submission"].(map[string]any); ok && submissionData != nil {
					workflowState, _ := submissionData["workflow_state"].(string)
					workflowState = strings.ToLower(workflowState)
					submittedAtTimestamp, _ := submissionData["submitted_at"].(string)
					hasScore := submissionData["score"] != nil || submissionData["grade"] != nil
					if workflowState == "graded" || workflowState == "complete" || workflowState == "submitted" || strings.TrimSpace(submittedAtTimestamp) != "" || hasScore {
						taskStatus = "done"
					}
					gradeStr = formatGradeString(submissionData, pointsPossible)
				}

				if err := upsertTaskRecord(app, tasksCollection, authRecord.Id, calendarID, assignmentTitle, dueAtTimestamp, taskStatus, gradeStr); err == nil {
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
						"calendars",
						"user = {:user} && name = {:name}",
						map[string]any{"user": authRecord.Id, "name": contextName},
					)
					if existingCalendarRecord != nil {
						calendarID = existingCalendarRecord.Id
					} else {
						newCalendarRecord := core.NewRecord(calendarsCollection)
						newCalendarRecord.Set("user", authRecord.Id)
						newCalendarRecord.Set("name", contextName)
						newCalendarRecord.Set("color", "#3b82f6")
						newCalendarRecord.Set("source", "canvas")
						newCalendarRecord.Set("visible", true)
						if err := app.Save(newCalendarRecord); err == nil {
							calendarID = newCalendarRecord.Id
						}
					}
					if calendarID != "" && normalizedCourseID != "" {
						canvasCourseToCalendarID[normalizedCourseID] = calendarID
					}
				}
			}

			taskStatus := "todo"
			gradeStr := ""
			if submissionsData, ok := plannerItem["submissions"].(map[string]any); ok && submissionsData != nil {
				isSubmitted, _ := submissionsData["submitted"].(bool)
				isGraded, _ := submissionsData["graded"].(bool)
				workflowState, _ := submissionsData["workflow_state"].(string)
				workflowState = strings.ToLower(workflowState)
				hasScore := submissionsData["score"] != nil || submissionsData["grade"] != nil
				if isSubmitted || isGraded || workflowState == "submitted" || workflowState == "graded" || workflowState == "complete" || hasScore {
					taskStatus = "done"
				}
				gradeStr = formatGradeString(submissionsData, 0)
			}

			if err := upsertTaskRecord(app, tasksCollection, authRecord.Id, calendarID, taskTitle, dueAtTimestamp, taskStatus, gradeStr); err == nil {
				upsertedTaskCount++
			} else {
				log.Printf("Failed to upsert task %s: %v", taskTitle, err)
			}
		}

		return event.JSON(http.StatusOK, CanvasSyncResponse{
			Success:       true,
			CoursesSynced: syncedCourseCount,
			TasksSynced:   upsertedTaskCount,
			Message:       fmt.Sprintf("Successfully synced %d courses and %d tasks from Canvas.", syncedCourseCount, upsertedTaskCount),
		})
	}
}
