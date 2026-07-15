package xai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const xaiOAuthBase = "https://api.x.ai/oauth"

// DeviceCodeResponse is returned when starting the device code flow.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse contains the tokens returned by xAI.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// StartDeviceCode initiates the OAuth device code flow with xAI.
func StartDeviceCode() (*DeviceCodeResponse, error) {
	payload := map[string]string{
		"client_id": "antisthenes-cli",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(xaiOAuthBase+"/device/code", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, string(data))
	}

	var dc DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return nil, err
	}
	return &dc, nil
}

// PollForAccessToken polls until the user completes authorization or the code expires.
func PollForAccessToken(deviceCode string, interval int) (*TokenResponse, error) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		payload := map[string]string{
			"client_id":   "antisthenes-cli",
			"device_code": deviceCode,
			"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		}

		body, _ := json.Marshal(payload)
		resp, err := http.Post(xaiOAuthBase+"/token", "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var token TokenResponse
			if err := json.Unmarshal(data, &token); err != nil {
				return nil, err
			}
			return &token, nil
		}

		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &errResp)

		if errResp.Error != "authorization_pending" {
			return nil, fmt.Errorf("xAI OAuth error: %s", errResp.Error)
		}
	}

	return nil, fmt.Errorf("device code expired")
}

// RefreshAccessToken refreshes an expired access token.
func RefreshAccessToken(refreshToken string) (*TokenResponse, error) {
	payload := map[string]string{
		"client_id":     "antisthenes-cli",
		"refresh_token": refreshToken,
		"grant_type":    "refresh_token",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(xaiOAuthBase+"/token", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s", string(data))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}
