package main

import (
	"backend/proxies"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type CanvasItemRequest struct {
	CourseID   string `json:"courseId"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	SourceLink string `json:"sourceLink"`
}

// handleCanvasItemDetails handles the incoming POST /api/canvas/item HTTP request.
func handleCanvasItemDetails(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		user := proxies.NewUser(authRecord)
		canvasURL := user.CanvasURL()
		canvasToken := user.CanvasToken()
		if canvasURL == "" || canvasToken == "" {
			return e.BadRequestError("Canvas not connected", nil)
		}

		var req CanvasItemRequest
		body, err := io.ReadAll(e.Request.Body)
		if err != nil {
			return e.BadRequestError("Failed to read body", err)
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return e.BadRequestError("Invalid JSON body", err)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		var foundItem map[string]any

		// 1. If SourceLink is provided, attempt direct item lookup (e.g. /courses/123/assignments/456)
		if req.SourceLink != "" {
			if idx := strings.Index(req.SourceLink, "/courses/"); idx != -1 {
				subPath := req.SourceLink[idx+len("/courses/"):]
				if qIdx := strings.IndexAny(subPath, "?#"); qIdx != -1 {
					subPath = subPath[:qIdx]
				}
				subPath = strings.Trim(subPath, "/")
				if subPath != "" {
					directURL := fmt.Sprintf("%s/api/v1/courses/%s", strings.TrimRight(canvasURL, "/"), subPath)
					httpReq, err := http.NewRequest("GET", directURL, nil)
					if err == nil {
						httpReq.Header.Set("Authorization", "Bearer "+canvasToken)
						httpReq.Header.Set("Accept", "application/json")
						resp, err := client.Do(httpReq)
						if err == nil && resp.StatusCode == http.StatusOK {
							var item map[string]any
							bodyBytes, errRead := io.ReadAll(resp.Body)
							if errRead == nil && json.Unmarshal(bodyBytes, &item) == nil {
								foundItem = item
							}
						}
						if resp != nil {
							resp.Body.Close()
						}
					}
				}
			}
		}

		// 2. Fallback to title search if direct lookup was not possible or didn't find the item
		if foundItem == nil && req.CourseID != "" {
			if req.Type == "deadline" {
				urlStr := fmt.Sprintf("%s/api/v1/courses/%s/assignments?per_page=100", canvasURL, req.CourseID)
				if len(req.Title) >= 3 {
					urlStr += "&search_term=" + url.QueryEscape(req.Title)
				}

				httpReq, _ := http.NewRequest("GET", urlStr, nil)
				httpReq.Header.Set("Authorization", "Bearer "+canvasToken)
				httpReq.Header.Set("Accept", "application/json")

				resp, err := client.Do(httpReq)
				if err == nil && resp.StatusCode == 200 {
					var items []map[string]any
					bodyBytes, _ := io.ReadAll(resp.Body)
					json.Unmarshal(bodyBytes, &items)
					for _, item := range items {
						if name, _ := item["name"].(string); strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(req.Title)) {
							foundItem = item
							break
						}
					}
				}
				if resp != nil {
					resp.Body.Close()
				}

				// If search_term returned nothing, try without search_term
				if foundItem == nil && len(req.Title) >= 3 {
					fallbackUrl := fmt.Sprintf("%s/api/v1/courses/%s/assignments?per_page=100", canvasURL, req.CourseID)
					httpReq2, _ := http.NewRequest("GET", fallbackUrl, nil)
					httpReq2.Header.Set("Authorization", "Bearer "+canvasToken)
					httpReq2.Header.Set("Accept", "application/json")
					resp2, err2 := client.Do(httpReq2)
					if err2 == nil && resp2.StatusCode == 200 {
						var items []map[string]any
						bodyBytes, _ := io.ReadAll(resp2.Body)
						json.Unmarshal(bodyBytes, &items)
						for _, item := range items {
							if name, _ := item["name"].(string); strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(req.Title)) {
								foundItem = item
								break
							}
						}
					}
					if resp2 != nil {
						resp2.Body.Close()
					}
				}

				// If still not found, check quizzes in this course
				if foundItem == nil {
					quizzesUrl := fmt.Sprintf("%s/api/v1/courses/%s/quizzes?per_page=100", canvasURL, req.CourseID)
					httpReqQ, _ := http.NewRequest("GET", quizzesUrl, nil)
					httpReqQ.Header.Set("Authorization", "Bearer "+canvasToken)
					httpReqQ.Header.Set("Accept", "application/json")
					respQ, errQ := client.Do(httpReqQ)
					if errQ == nil && respQ.StatusCode == 200 {
						var items []map[string]any
						bodyBytes, _ := io.ReadAll(respQ.Body)
						json.Unmarshal(bodyBytes, &items)
						for _, item := range items {
							if title, _ := item["title"].(string); strings.EqualFold(strings.TrimSpace(title), strings.TrimSpace(req.Title)) {
								foundItem = item
								break
							}
						}
					}
					if respQ != nil {
						respQ.Body.Close()
					}
				}
			} else if req.Type == "announcement" {
				urlStr := fmt.Sprintf("%s/api/v1/courses/%s/discussion_topics?only_announcements=true&per_page=100", canvasURL, req.CourseID)
				if len(req.Title) >= 3 {
					urlStr += "&search_term=" + url.QueryEscape(req.Title)
				}

				httpReq, _ := http.NewRequest("GET", urlStr, nil)
				httpReq.Header.Set("Authorization", "Bearer "+canvasToken)
				httpReq.Header.Set("Accept", "application/json")

				resp, err := client.Do(httpReq)
				if err == nil && resp.StatusCode == 200 {
					var items []map[string]any
					bodyBytes, _ := io.ReadAll(resp.Body)
					json.Unmarshal(bodyBytes, &items)
					for _, item := range items {
						if title, _ := item["title"].(string); strings.EqualFold(strings.TrimSpace(title), strings.TrimSpace(req.Title)) {
							foundItem = item
							break
						}
					}
				}
				if resp != nil {
					resp.Body.Close()
				}
			}
		}

		if foundItem == nil {
			return e.NotFoundError("Item not found on Canvas", nil)
		}

		if msg, ok := foundItem["message"].(string); ok {
			foundItem["message"] = cleanCanvasHTML(msg)
		}
		if desc, ok := foundItem["description"].(string); ok {
			foundItem["description"] = cleanCanvasHTML(desc)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"success": true,
			"data": foundItem,
		})
	}
}
