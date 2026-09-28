package google

import (
	"backend/proxies"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

/*
getAuth extracts the authenticated user record from the PocketBase RequestEvent.
*/
func getAuth(app core.App, event *core.RequestEvent) (*core.Record, error) {
	if event.Auth == nil {
		return nil, event.UnauthorizedError("Authentication required", nil)
	}

	return event.Auth, nil
}

func getAuthProx(app core.App, event *core.RequestEvent) (*proxies.User, error) {
	if event.Auth == nil {
		return nil, event.UnauthorizedError("Authentication required", nil)
	}

	return proxies.NewUser(event.Auth), nil
}

/*
GetAuth is an exported version of getAuth for use across packages if needed.
*/
func GetAuth(app core.App, event *core.RequestEvent) (*core.Record, error) {
	return getAuth(app, event)
}

/*
refreshGoogleToken exchanges a user's stored Google OAuth refresh token for a new access token.
AI GEN: YES
HUMAN AUDIT: NO
*/
func refreshGoogleToken(app core.App, authRecord *core.Record) (string, error) {
	user := proxies.NewUser(authRecord)
	refreshToken := user.GoogleRefreshToken()
	if refreshToken == "" {
		return "", fmt.Errorf("no refresh token available")
	}

	usersCollection, err := app.FindCollectionByNameOrId(proxies.CollectionUsers)
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

	user.SetGoogleAccessToken(tokenResp.AccessToken)
	user.SetGoogleConnected(true)
	if tokenResp.ExpiresIn > 0 {
		user.SetGoogleTokenExpiry(time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second))
	}
	_ = app.Save(user)

	return tokenResp.AccessToken, nil
}

/*
refreshGoogleTokenProx exchanges a user's stored Google OAuth refresh token for a new access token.
AI GEN: YES
HUMAN AUDIT: NO
*/
func refreshGoogleTokenProx(app core.App, authUser *proxies.User) (string, error) {
	refreshToken := authUser.GoogleRefreshToken()
	if refreshToken == "" {
		return "", fmt.Errorf("no refresh token available")
	}

	usersCollection, err := app.FindCollectionByNameOrId(proxies.CollectionUsers)
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

	authUser.SetGoogleAccessToken(tokenResp.AccessToken)
	authUser.SetGoogleConnected(true)
	if tokenResp.ExpiresIn > 0 {
		authUser.SetGoogleTokenExpiry(time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second))
	}
	_ = app.Save(authUser)

	return tokenResp.AccessToken, nil
}
