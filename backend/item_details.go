package main

import (
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
	CourseID string `json:"courseId"`
	Title    string `json:"title"`
	Type     string `json:"type"`
}

func getCanvasItemDetails(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		authRecord, err := getAuth(app, e)
		if err != nil {
			return err
		}

		canvasURL := authRecord.GetString("canvas_url")
		canvasToken := authRecord.GetString("canvas_token")
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

		if req.Type == "deadline" {
			urlStr := fmt.Sprintf("%s/api/v1/courses/%s/assignments?per_page=100", canvasURL, req.CourseID)
			if len(req.Title) >= 3 {
				urlStr += "&search_term=" + url.QueryEscape(req.Title)
			}
			
			httpReq, _ := http.NewRequest("GET", urlStr, nil)
			httpReq.Header.Set("Authorization", "Bearer "+canvasToken)
			
			resp, err := client.Do(httpReq)
			if err == nil && resp.StatusCode == 200 {
				var items []map[string]any
				bodyBytes, _ := io.ReadAll(resp.Body)
				json.Unmarshal(bodyBytes, &items)
				for _, item := range items {
					if name, _ := item["name"].(string); strings.TrimSpace(name) == req.Title {
						foundItem = item
						break
					}
				}
			}
			if resp != nil {
				resp.Body.Close()
			}
		} else if req.Type == "announcement" {
			urlStr := fmt.Sprintf("%s/api/v1/courses/%s/discussion_topics?only_announcements=true&per_page=100", canvasURL, req.CourseID)
			if len(req.Title) >= 3 {
				urlStr += "&search_term=" + url.QueryEscape(req.Title)
			}
			
			httpReq, _ := http.NewRequest("GET", urlStr, nil)
			httpReq.Header.Set("Authorization", "Bearer "+canvasToken)
			
			resp, err := client.Do(httpReq)
			if err == nil && resp.StatusCode == 200 {
				var items []map[string]any
				bodyBytes, _ := io.ReadAll(resp.Body)
				json.Unmarshal(bodyBytes, &items)
				for _, item := range items {
					if title, _ := item["title"].(string); strings.TrimSpace(title) == req.Title {
						foundItem = item
						break
					}
				}
			}
			if resp != nil {
				resp.Body.Close()
			}
		}

		if foundItem == nil {
			return e.NotFoundError("Item not found on Canvas", nil)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"success": true,
			"data": foundItem,
		})
	}
}
