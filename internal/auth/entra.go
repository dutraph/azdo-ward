// Package auth obtains Azure Entra ID access tokens for the Azure DevOps
// resource by shelling out to the Azure CLI (`az`). This is the path for
// organizations whose policy blocks Personal Access Tokens and forces
// Entra-backed authentication.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// azureDevOpsResource is the fixed application (resource) ID of Azure
// DevOps. Tokens minted for this resource are accepted by dev.azure.com
// and auditservice.dev.azure.com.
const azureDevOpsResource = "499b84ac-1321-427f-aa17-267ca6975798"

// azToken mirrors the JSON shape of `az account get-access-token -o json`.
type azToken struct {
	AccessToken string `json:"accessToken"`
	ExpiresOn   string `json:"expiresOn"`   // local time, no offset (legacy)
	Expires_On  int64  `json:"expires_on"`  // unix seconds (newer az)
	TokenType   string `json:"tokenType"`
}

// entraCache caches the most recent token until shortly before it expires
// so we don't fork `az` on every API call.
type entraCache struct {
	mu      sync.Mutex
	token   string
	expires time.Time
}

var cache entraCache

// EntraToken returns a valid Entra access token for Azure DevOps, fetching
// (and caching) it via the Azure CLI. It satisfies the provider signature
// expected by api.BearerAuth.
func EntraToken(ctx context.Context) (string, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.token != "" && time.Until(cache.expires) > 2*time.Minute {
		return cache.token, nil
	}

	tok, exp, err := fetchAZ(ctx)
	if err != nil {
		return "", err
	}
	cache.token = tok
	cache.expires = exp
	return tok, nil
}

func fetchAZ(ctx context.Context) (string, time.Time, error) {
	if _, err := exec.LookPath("az"); err != nil {
		return "", time.Time{}, fmt.Errorf("the Azure CLI ('az') was not found on PATH — install it and run 'az login' to use Entra auth")
	}

	cmd := exec.CommandContext(ctx, "az", "account", "get-access-token",
		"--resource", azureDevOpsResource, "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr(err))
		if msg == "" {
			msg = err.Error()
		}
		return "", time.Time{}, fmt.Errorf("az account get-access-token failed (run 'az login'): %s", msg)
	}

	var t azToken
	if err := json.Unmarshal(out, &t); err != nil {
		return "", time.Time{}, fmt.Errorf("parsing az token output: %w", err)
	}
	if t.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("az returned an empty access token")
	}

	exp := parseExpiry(t)
	return t.AccessToken, exp, nil
}

// parseExpiry derives an expiry time from whichever field az populated,
// falling back to a conservative 45-minute lifetime.
func parseExpiry(t azToken) time.Time {
	if t.Expires_On > 0 {
		return time.Unix(t.Expires_On, 0)
	}
	if t.ExpiresOn != "" {
		// Newer az emits RFC3339; older emits "2006-01-02 15:04:05.000000"
		// in local time. Try both.
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05.000000", "2006-01-02 15:04:05"} {
			if ts, err := time.ParseInLocation(layout, t.ExpiresOn, time.Local); err == nil {
				return ts
			}
		}
	}
	return time.Now().Add(45 * time.Minute)
}

func stderr(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return ""
}
