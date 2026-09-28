package google

import (
	"backend/proxies"
	"bytes"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// =============================================================================
// Authenticated Google API Request Helper with Auto-Refresh & HTTP/2 Pooling
// =============================================================================

var (
	googleSharedHTTPClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
		},
		Timeout: 25 * time.Second,
	}
	GoogleSharedHTTPClient = googleSharedHTTPClient
	googleTokenRefreshMu   sync.Mutex
	googleWriteLimiter     = time.NewTicker(time.Second / 4) // Strict cap: 4 writes / sec
)

type GetGoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

/*
 */
func makeGoogleAPIRequestProx(app core.App, authUser *proxies.User, method string, endpoint string, body []byte) (*http.Response, string, error) {
	token := authUser.GoogleAccessToken()
	expiry := authUser.GoogleTokenExpiryTime()
	//DOUBLE CHECK THIS
	isExpired := !expiry.IsZero() && time.Now().Add(1*time.Minute).After(expiry)

	if token == "" || isExpired {
		if authUser.GoogleRefreshToken() != "" {
			googleTokenRefreshMu.Lock()
			refreshed, err := refreshGoogleTokenProx(app, authUser)
			googleTokenRefreshMu.Unlock()
			if err == nil && refreshed != "" {
				token = refreshed
			}
		}
	}

	if token == "" {
		return nil, "", fmt.Errorf("no valid Google OAuth token found")
	}

	// Enforce strict rate limit on write requests (POST, PATCH, DELETE, PUT): max 9 writes / second
	if method != http.MethodGet && method != http.MethodHead {
		<-googleWriteLimiter.C
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, endpoint, bodyReader)
	if err != nil {
		return nil, token, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := googleSharedHTTPClient.Do(req)
	if err != nil {
		return nil, token, err
	}

	// If token expired (401), attempt refresh and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		googleTokenRefreshMu.Lock()
		if authUser.GoogleRefreshToken() != "" {
			googleTokenRefreshMu.Lock()
			refreshed, err := refreshGoogleTokenProx(app, authUser)
			googleTokenRefreshMu.Unlock()
			if err == nil && refreshed != "" {
				token = refreshed
			}
		}

		if method != http.MethodGet && method != http.MethodHead {
			<-googleWriteLimiter.C
		}

		if len(body) > 0 {
			bodyReader = bytes.NewReader(body)
		}
		retryReq, err := http.NewRequest(method, endpoint, bodyReader)
		if err != nil {
			return nil, token, err
		}
		retryReq.Header.Set("Authorization", "Bearer "+token)
		retryReq.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			retryReq.Header.Set("Content-Type", "application/json")
		}

		resp, err = googleSharedHTTPClient.Do(retryReq)
		if err != nil {
			return nil, token, err
		}
	}

	// If rate limited (429 or 403 quota/rateLimitExceeded), back off and retry up to 3 times
	for attempt := 1; attempt <= 3 && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden); attempt++ {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		backoff := time.Duration(attempt) * 1000 * time.Millisecond
		log.Printf("[RateLimit] Google API rate limited (%d). Backing off %v (attempt %d/3)...", resp.StatusCode, backoff, attempt)
		time.Sleep(backoff)

		if method != http.MethodGet && method != http.MethodHead {
			<-googleWriteLimiter.C
		}

		if len(body) > 0 {
			bodyReader = bytes.NewReader(body)
		}
		retryReq, err := http.NewRequest(method, endpoint, bodyReader)
		if err != nil {
			return nil, token, err
		}
		retryReq.Header.Set("Authorization", "Bearer "+token)
		retryReq.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			retryReq.Header.Set("Content-Type", "application/json")
		}

		resp, err = googleSharedHTTPClient.Do(retryReq)
		if err != nil {
			return nil, token, err
		}
	}

	return resp, token, nil
}

func makeGoogleAPIRequest(app core.App, authRecord *core.Record, method string, endpoint string, body []byte) (*http.Response, string, error) {
	authUser := proxies.NewUser(authRecord)
	token := authUser.GoogleAccessToken()
	expiry := authUser.GoogleTokenExpiryTime()
	isExpired := !expiry.IsZero() && time.Now().Add(1*time.Minute).After(expiry)

	if token == "" || isExpired {
		if authUser.GoogleRefreshToken() != "" {
			googleTokenRefreshMu.Lock()
			refreshed, err := refreshGoogleToken(app, authRecord)
			googleTokenRefreshMu.Unlock()
			if err == nil && refreshed != "" {
				token = refreshed
			}
		}
	}
	if token == "" {
		return nil, "", fmt.Errorf("no valid Google OAuth token found")
	}

	// Enforce strict rate limit on write requests (POST, PATCH, DELETE, PUT): max 9 writes / second
	if method != http.MethodGet && method != http.MethodHead {
		<-googleWriteLimiter.C
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, endpoint, bodyReader)
	if err != nil {
		return nil, token, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := googleSharedHTTPClient.Do(req)
	if err != nil {
		return nil, token, err
	}

	// If token expired (401), attempt refresh and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		googleTokenRefreshMu.Lock()
		refreshedUser := proxies.NewUser(authRecord)
		currentToken := refreshedUser.GoogleAccessToken()
		if currentToken != "" && currentToken != token {
			token = currentToken
			googleTokenRefreshMu.Unlock()
		} else {
			refreshed, errRef := refreshGoogleToken(app, authRecord)
			googleTokenRefreshMu.Unlock()
			if errRef != nil || refreshed == "" {
				return nil, token, fmt.Errorf("google token expired and refresh failed: %v", errRef)
			}
			token = refreshed
		}

		if method != http.MethodGet && method != http.MethodHead {
			<-googleWriteLimiter.C
		}

		if len(body) > 0 {
			bodyReader = bytes.NewReader(body)
		}
		retryReq, err := http.NewRequest(method, endpoint, bodyReader)
		if err != nil {
			return nil, token, err
		}
		retryReq.Header.Set("Authorization", "Bearer "+token)
		retryReq.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			retryReq.Header.Set("Content-Type", "application/json")
		}

		resp, err = googleSharedHTTPClient.Do(retryReq)
		if err != nil {
			return nil, token, err
		}
	}

	// If rate limited (429 or 403 quota/rateLimitExceeded), back off and retry up to 3 times
	for attempt := 1; attempt <= 3 && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden); attempt++ {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		backoff := time.Duration(attempt) * 1000 * time.Millisecond
		log.Printf("[RateLimit] Google API rate limited (%d). Backing off %v (attempt %d/3)...", resp.StatusCode, backoff, attempt)
		time.Sleep(backoff)

		if method != http.MethodGet && method != http.MethodHead {
			<-googleWriteLimiter.C
		}

		if len(body) > 0 {
			bodyReader = bytes.NewReader(body)
		}
		retryReq, err := http.NewRequest(method, endpoint, bodyReader)
		if err != nil {
			return nil, token, err
		}
		retryReq.Header.Set("Authorization", "Bearer "+token)
		retryReq.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			retryReq.Header.Set("Content-Type", "application/json")
		}

		resp, err = googleSharedHTTPClient.Do(retryReq)
		if err != nil {
			return nil, token, err
		}
	}

	return resp, token, nil
}
