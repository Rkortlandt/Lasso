package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

type GoogleEventItem struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type GoogleCalendarEntry struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Primary bool   `json:"primary"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func main() {
	dbPathFlag := flag.String("db", "", "Path to PocketBase data.db (defaults to auto-detecting backend/pb_data/data.db)")
	recreateFlag := flag.Bool("recreate", false, "Delete old Lasso calendar and create a brand new clean one in Google Calendar")
	deleteCalFlag := flag.Bool("delete-calendar", false, "Delete the secondary Lasso calendar entirely instead of clearing its events")
	clearRegistryFlag := flag.Bool("clear-registry", true, "Also clear local google_sync_registry records for the user")
	concurrencyFlag := flag.Int("concurrency", 1, "Number of concurrent workers for deleting events")
	flag.Parse()

	dbPath := *dbPathFlag
	if dbPath == "" {
		candidates := []string{
			"backend/pb_data/data.db",
			"../backend/pb_data/data.db",
			"../../backend/pb_data/data.db",
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				abs, _ := filepath.Abs(cand)
				dbPath = abs
				break
			}
		}
	}

	if dbPath == "" {
		log.Fatalf("Could not locate backend/pb_data/data.db. Please provide -db path/to/data.db")
	}

	log.Printf("[1/5] Reading user Google credentials from SQLite: %s", dbPath)
	dbURI := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite", dbURI)
	if err != nil {
		log.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Query user with google_connected = 1
	var userID, email, accessToken, refreshToken string
	var googleConnected bool
	err = db.QueryRow(`
		SELECT id, email, google_connected, google_access_token, google_refresh_token 
		FROM users 
		WHERE google_connected = 1 AND google_refresh_token != '' 
		LIMIT 1
	`).Scan(&userID, &email, &googleConnected, &accessToken, &refreshToken)
	if err != nil {
		log.Fatalf("No Google-connected user found in database: %v", err)
	}
	log.Printf("Found user %s (%s)", email, userID)

	// Query OAuth config
	var oauth2JSON sql.NullString
	_ = db.QueryRow(`SELECT oauth2 FROM _collections WHERE name = 'users'`).Scan(&oauth2JSON)

	var clientID, clientSecret string
	if oauth2JSON.Valid && oauth2JSON.String != "" {
		var colData struct {
			Providers []struct {
				Name         string `json:"name"`
				ClientId     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
			} `json:"providers"`
		}
		if errDec := json.Unmarshal([]byte(oauth2JSON.String), &colData); errDec == nil {
			for _, p := range colData.Providers {
				if p.Name == "google" {
					clientID = p.ClientId
					clientSecret = p.ClientSecret
					break
				}
			}
		}
	}

	// Validate / Refresh Google access token
	token := accessToken
	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: 90 * time.Second,
	}

	log.Printf("[2/5] Validating Google Calendar access token...")
	calListURL := "https://www.googleapis.com/calendar/v3/users/me/calendarList?minAccessRole=writer"
	req, _ := http.NewRequest("GET", calListURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)

	if err != nil || (resp != nil && resp.StatusCode == http.StatusUnauthorized) {
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("Access token expired or unauthorized. Refreshing token via Google OAuth2...")
		if clientID == "" || clientSecret == "" || refreshToken == "" {
			log.Fatalf("Cannot refresh token: missing clientId, clientSecret, or refreshToken in database")
		}

		data := url.Values{}
		data.Set("client_id", clientID)
		data.Set("client_secret", clientSecret)
		data.Set("refresh_token", refreshToken)
		data.Set("grant_type", "refresh_token")

		refResp, errRef := httpClient.PostForm("https://oauth2.googleapis.com/token", data)
		if errRef != nil {
			log.Fatalf("Token refresh HTTP request failed: %v", errRef)
		}
		defer refResp.Body.Close()

		if refResp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(refResp.Body)
			log.Fatalf("Token refresh failed (status %d): %s", refResp.StatusCode, string(b))
		}

		var tr RefreshTokenResponse
		if errDec := json.NewDecoder(refResp.Body).Decode(&tr); errDec != nil || tr.AccessToken == "" {
			log.Fatalf("Failed to decode token response: %v", errDec)
		}
		token = tr.AccessToken
		log.Printf("Successfully refreshed Google OAuth2 token.")

		// Retry calendarList with fresh token
		req, _ = http.NewRequest("GET", calListURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = httpClient.Do(req)
		if err != nil {
			log.Fatalf("Failed to query calendarList with fresh token: %v", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		log.Fatalf("calendarList request failed (status %d): %s", resp.StatusCode, string(b))
	}

	var calListResp struct {
		Items []GoogleCalendarEntry `json:"items"`
	}
	if errDec := json.NewDecoder(resp.Body).Decode(&calListResp); errDec != nil {
		log.Fatalf("Failed to decode calendar list: %v", errDec)
	}

	var lassoCalendars []GoogleCalendarEntry
	for _, c := range calListResp.Items {
		if strings.EqualFold(strings.TrimSpace(c.Summary), "Lasso") {
			lassoCalendars = append(lassoCalendars, c)
		}
	}

	log.Printf("[3/5] Checking existing 'Lasso' calendar(s) in Google Calendar: found %d", len(lassoCalendars))
	for i, c := range lassoCalendars {
		log.Printf("  [%d] ID: %s | Summary: %s", i+1, c.ID, c.Summary)
	}

	if *recreateFlag {
		for _, c := range lassoCalendars {
			log.Printf("Deleting old calendar %s from Google Calendar...", c.ID)
			delURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s", url.PathEscape(c.ID))
			delReq, _ := http.NewRequest("DELETE", delURL, nil)
			delReq.Header.Set("Authorization", "Bearer "+token)
			delResp, errDel := httpClient.Do(delReq)
			if errDel == nil {
				io.Copy(io.Discard, delResp.Body)
				delResp.Body.Close()
			}
		}

		log.Printf("[4/5] Creating brand new 'Lasso' secondary calendar in Google Calendar...")
		createPayload := map[string]string{
			"summary":     "Lasso",
			"description": "Academic deadlines, schedules, and coursework synced from Lasso",
			"timeZone":    "UTC",
		}
		createBody, _ := json.Marshal(createPayload)
		createReq, _ := http.NewRequest("POST", "https://www.googleapis.com/calendar/v3/calendars", bytes.NewReader(createBody))
		createReq.Header.Set("Authorization", "Bearer "+token)
		createReq.Header.Set("Content-Type", "application/json")
		createResp, errCr := httpClient.Do(createReq)
		if errCr != nil {
			log.Fatalf("Failed to create Google calendar: %v", errCr)
		}
		defer createResp.Body.Close()

		if createResp.StatusCode != http.StatusOK && createResp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(createResp.Body)
			log.Fatalf("Failed to create Google calendar (status %d): %s", createResp.StatusCode, string(b))
		}

		var newCreatedCal GoogleCalendarEntry
		if errDec := json.NewDecoder(createResp.Body).Decode(&newCreatedCal); errDec != nil {
			log.Fatalf("Failed to decode created calendar: %v", errDec)
		}
		log.Printf("Successfully created brand new 'Lasso' secondary calendar in Google! ID: %s", newCreatedCal.ID)

		// Update PocketBase calendars table
		rwDB, errRW := sql.Open("sqlite", fmt.Sprintf("file:%s?_busy_timeout=10000", dbPath))
		if errRW == nil {
			res, _ := rwDB.Exec("UPDATE calendars SET calendar_id = ? WHERE user = ? AND name = 'Lasso'", newCreatedCal.ID, userID)
			rows, _ := res.RowsAffected()
			if rows == 0 {
				_, _ = rwDB.Exec(`
					INSERT INTO calendars (id, user, name, color, source, visible, calendar_id, created, updated)
					VALUES (substr(hex(randomblob(8)), 1, 15), ?, 'Lasso', '#515151', 'google', 1, ?, datetime('now'), datetime('now'))
				`, userID, newCreatedCal.ID)
			}
			rwDB.Close()
		}
		log.Printf("Updated PocketBase calendars record with new Google Calendar ID.")
	} else if *deleteCalFlag {
		for _, c := range lassoCalendars {
			log.Printf("Deleting secondary calendar %s from Google Calendar...", c.ID)
			delURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s", url.PathEscape(c.ID))
			delReq, _ := http.NewRequest("DELETE", delURL, nil)
			delReq.Header.Set("Authorization", "Bearer "+token)
			delResp, errDel := httpClient.Do(delReq)
			if errDel != nil {
				log.Printf("Error deleting calendar %s: %v", c.ID, errDel)
			} else {
				io.Copy(io.Discard, delResp.Body)
				delResp.Body.Close()
				log.Printf("Successfully deleted Google Calendar %s (status: %d)", c.ID, delResp.StatusCode)
			}
		}
	} else {
		// Event deletion mode (keeps the calendar ID intact, wipes all events inside)
		for _, cal := range lassoCalendars {
			log.Printf("Fetching events from Lasso calendar (%s)...", cal.ID)
			var allEvents []GoogleEventItem
			pageToken := ""

			for {
				evURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?maxResults=2500&showDeleted=false", url.PathEscape(cal.ID))
				if pageToken != "" {
					evURL += "&pageToken=" + url.QueryEscape(pageToken)
				}
				evReq, _ := http.NewRequest("GET", evURL, nil)
				evReq.Header.Set("Authorization", "Bearer "+token)
				evResp, errEv := httpClient.Do(evReq)
				if errEv != nil {
					log.Fatalf("Failed to fetch events: %v", errEv)
				}

				if evResp.StatusCode != http.StatusOK {
					b, _ := io.ReadAll(evResp.Body)
					evResp.Body.Close()
					log.Fatalf("Failed to fetch events (status %d): %s", evResp.StatusCode, string(b))
				}

				var evListResp struct {
					Items         []GoogleEventItem `json:"items"`
					NextPageToken string            `json:"nextPageToken"`
				}
				_ = json.NewDecoder(evResp.Body).Decode(&evListResp)
				evResp.Body.Close()

				for _, it := range evListResp.Items {
					if it.Status != "cancelled" {
						allEvents = append(allEvents, it)
					}
				}

				if evListResp.NextPageToken == "" {
					break
				}
				pageToken = evListResp.NextPageToken
			}

			log.Printf("[4/5] Found %d active events in calendar '%s'. Purging concurrently with %d workers...", len(allEvents), cal.ID, *concurrencyFlag)
			if len(allEvents) == 0 {
				log.Printf("Calendar '%s' is already completely empty.", cal.ID)
				continue
			}

			concurrency := *concurrencyFlag
			if concurrency < 1 {
				concurrency = 1
			}
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup
			var deletedCount int64
			var errorCount int64

			startPurge := time.Now()
			for _, item := range allEvents {
				wg.Add(1)
				sem <- struct{}{}
				go func(ev GoogleEventItem) {
					defer func() {
						<-sem
						wg.Done()
					}()

					delURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s", url.PathEscape(cal.ID), url.PathEscape(ev.ID))
					delReq, _ := http.NewRequest("DELETE", delURL, nil)
					delReq.Header.Set("Authorization", "Bearer "+token)
					delResp, errD := httpClient.Do(delReq)
					if errD != nil {
						atomic.AddInt64(&errorCount, 1)
						return
					}
					io.Copy(io.Discard, delResp.Body)
					delResp.Body.Close()

					if delResp.StatusCode == http.StatusOK || delResp.StatusCode == http.StatusNoContent || delResp.StatusCode == http.StatusNotFound || delResp.StatusCode == http.StatusGone {
						c := atomic.AddInt64(&deletedCount, 1)
						if c%50 == 0 || c == int64(len(allEvents)) {
							log.Printf("  Progress: deleted %d / %d events...", c, len(allEvents))
						}
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				}(item)
			}
			wg.Wait()
			log.Printf("Purge complete for calendar '%s': %d events deleted (%d errors) in %s.", cal.ID, deletedCount, errorCount, time.Since(startPurge).Round(time.Millisecond))
		}
	}

	// [5/5] Clear local database registry records if requested
	if *clearRegistryFlag {
		log.Printf("[5/5] Clearing local google_sync_registry records from SQLite...")
		// Open in read-write mode
		rwDB, errRW := sql.Open("sqlite", fmt.Sprintf("file:%s?_busy_timeout=10000", dbPath))
		if errRW == nil {
			res, errDel := rwDB.Exec("DELETE FROM google_sync_registry WHERE user = ?", userID)
			if errDel != nil {
				log.Printf("Notice: Could not delete from google_sync_registry: %v", errDel)
			} else {
				rows, _ := res.RowsAffected()
				log.Printf("Successfully removed %d cached records; cleared calendars and sync_status for user %s", rows, email)
			}

			// Also reset sync_status for google_export
			_, _ = rwDB.Exec(`
				UPDATE sync_status 
				SET google_export_status = 'idle', 
				    google_export_synced_at = '', 
				    google_export_feedback = 'Ready for fresh sync.' 
				WHERE user = ?
			`, userID)
			rwDB.Close()
		}
	}

	log.Printf("All done!")
}
