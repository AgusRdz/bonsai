package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceFlowConfig holds the OAuth App credentials for the device flow.
type DeviceFlowConfig struct {
	ClientID string
	// GitHub scope, e.g. "repo,read:user"
	Scope string
	// DeviceCodeURL e.g. "https://github.com/login/device/code"
	DeviceCodeURL string
	// TokenURL e.g. "https://github.com/login/oauth/access_token"
	TokenURL string
	// Host e.g. "github.com"
	Host string
}

// DeviceCodeResponse is the response from the first device flow request.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceFlowProgress carries status updates to the caller during polling.
type DeviceFlowProgress struct {
	// UserCode and VerificationURI are set after the first request.
	UserCode        string
	VerificationURI string
	// Done is set to true when the token has been obtained.
	Done  bool
	Token *Token
	// Err is set on failure.
	Err error
}

// GitHubConfig returns the device flow config for GitHub.com.
// clientID should come from BONSAI_GITHUB_CLIENT_ID or the embedded default.
func GitHubConfig(clientID string) DeviceFlowConfig {
	return DeviceFlowConfig{
		ClientID:      clientID,
		Scope:         "repo,read:user,read:org",
		DeviceCodeURL: "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
		Host:          "github.com",
	}
}

// GitLabConfig returns the device flow config for GitLab.com.
func GitLabConfig(clientID string) DeviceFlowConfig {
	return DeviceFlowConfig{
		ClientID:      clientID,
		Scope:         "api read_user",
		DeviceCodeURL: "https://gitlab.com/oauth/authorize_device",
		TokenURL:      "https://gitlab.com/oauth/token",
		Host:          "gitlab.com",
	}
}

// StartDeviceFlow initiates the device flow and returns the first response so
// the caller can display the user_code. Call PollDeviceFlow to wait for completion.
func StartDeviceFlow(ctx context.Context, cfg DeviceFlowConfig) (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("scope", cfg.Scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request: %w", err)
	}
	defer resp.Body.Close()

	var dcr DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		return nil, fmt.Errorf("device code parse: %w", err)
	}
	if dcr.DeviceCode == "" {
		return nil, fmt.Errorf("no device_code in response")
	}
	return &dcr, nil
}

// PollDeviceFlow polls the token endpoint until the user completes auth,
// the token expires, or ctx is cancelled. progress receives updates.
// The final Token is returned; use DefaultManager.Set() to persist it.
func PollDeviceFlow(ctx context.Context, cfg DeviceFlowConfig, dcr *DeviceCodeResponse) (*Token, error) {
	interval := time.Duration(dcr.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dcr.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("device code expired")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		tok, err := pollOnce(ctx, cfg, dcr.DeviceCode)
		if err == errAuthPending {
			continue
		}
		if err == errSlowDown {
			interval += 5 * time.Second
			continue
		}
		if err != nil {
			return nil, err
		}
		return tok, nil
	}
}

var errAuthPending = fmt.Errorf("authorization_pending")
var errSlowDown = fmt.Errorf("slow_down")

func pollOnce(ctx context.Context, cfg DeviceFlowConfig, deviceCode string) (*Token, error) {
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	switch raw.Error {
	case "authorization_pending":
		return nil, errAuthPending
	case "slow_down":
		return nil, errSlowDown
	case "expired_token":
		return nil, fmt.Errorf("device code expired")
	case "access_denied":
		return nil, fmt.Errorf("access denied by user")
	case "":
		// success
	default:
		return nil, fmt.Errorf("OAuth error: %s", raw.Error)
	}

	if raw.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in response")
	}

	var scopes []string
	for _, s := range strings.Split(raw.Scope, ",") {
		if s = strings.TrimSpace(s); s != "" {
			scopes = append(scopes, s)
		}
	}

	return &Token{
		Host:        cfg.Host,
		AccessToken: raw.AccessToken,
		TokenType:   raw.TokenType,
		Scopes:      scopes,
	}, nil
}
