// Package api is a hand-rolled REST client for the Azure DevOps Audit
// service. No heavy SDK — just net/http, in the style of the azdo-tui /
// awsso-tui house pattern. It supports two auth modes: a Personal Access
// Token (HTTP basic) or an Azure Entra ID bearer token (for orgs whose
// policy blocks PAT auth).
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// auditBaseURL is the dedicated audit host. Note it is auditservice.dev.azure.com,
// NOT the regular dev.azure.com instance.
const auditBaseURL = "https://auditservice.dev.azure.com"

// apiVersion is pinned to the only version that exposes the auditlog query.
const apiVersion = "7.1-preview.1"

// Authorizer returns the value for the Authorization header on each
// request. It takes a context so bearer providers can refresh tokens.
type Authorizer func(ctx context.Context) (string, error)

// PATAuth builds an Authorizer for a Personal Access Token. Azure DevOps
// PAT auth is HTTP Basic with an empty username and the PAT as password.
func PATAuth(pat string) Authorizer {
	h := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pat))
	return func(context.Context) (string, error) { return h, nil }
}

// BearerAuth builds an Authorizer from a token provider (e.g. one that
// shells out to `az account get-access-token`). The provider is called
// per request, so it should cache internally.
func BearerAuth(provider func(ctx context.Context) (string, error)) Authorizer {
	return func(ctx context.Context) (string, error) {
		tok, err := provider(ctx)
		if err != nil {
			return "", err
		}
		return "Bearer " + strings.TrimSpace(tok), nil
	}
}

// Client talks to one Azure DevOps organization's audit log.
type Client struct {
	org  string
	auth Authorizer
	http *http.Client
	base string // overridable for tests
}

// New builds a client for the given organization with the chosen auth.
func New(org string, auth Authorizer) *Client {
	return &Client{
		org:  org,
		auth: auth,
		base: auditBaseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
			// Do NOT follow redirects: when auth is rejected Azure DevOps
			// answers a 302 to an interactive sign-in page. Following it
			// would mask the real problem behind an HTML login body, so we
			// keep the 302 and turn it into a clear error below.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// QueryOptions are the optional filters the Audit API accepts server-side.
// Anything left zero is omitted from the request.
type QueryOptions struct {
	StartTime         time.Time
	EndTime           time.Time
	BatchSize         int
	ContinuationToken string
	SkipAggregation   bool
}

// Query fetches a single batch of audit entries.
func (c *Client) Query(ctx context.Context, opts QueryOptions) (*QueryResult, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/_apis/audit/auditlog", c.base, url.PathEscape(c.org)))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("api-version", apiVersion)
	if !opts.StartTime.IsZero() {
		q.Set("startTime", opts.StartTime.UTC().Format(time.RFC3339))
	}
	if !opts.EndTime.IsZero() {
		q.Set("endTime", opts.EndTime.UTC().Format(time.RFC3339))
	}
	if opts.BatchSize > 0 {
		q.Set("batchSize", strconv.Itoa(opts.BatchSize))
	}
	if opts.ContinuationToken != "" {
		q.Set("continuationToken", opts.ContinuationToken)
	}
	if opts.SkipAggregation {
		q.Set("skipAggregation", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	authz, err := c.auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("building credentials: %w", err)
	}
	req.Header.Set("Authorization", authz)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("audit request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))

	// Auth rejection: a 302 to _signin (or a 203 "non-authoritative" login
	// page) means the credential was not accepted. Give an actionable error
	// rather than a JSON-parse failure.
	if resp.StatusCode == http.StatusFound ||
		resp.StatusCode == http.StatusNonAuthoritativeInfo ||
		isSignInRedirect(resp) {
		return nil, &AuthError{Status: resp.StatusCode, Location: resp.Header.Get("Location")}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &AuthError{Status: resp.StatusCode, Body: snippet(body)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("audit API returned %s: %s", resp.Status, snippet(body))
	}

	// The live API returns the result at the top level
	// ({"decoratedAuditLogEntries":[...]}) even though the published docs
	// show it wrapped in {"value":{...}}. Parse the flat form first and
	// fall back to the envelope so we're robust to both shapes.
	var res QueryResult
	if err := json.Unmarshal(body, &res); err != nil {
		// A 200 that isn't JSON is almost always a sign-in page slipping
		// through; surface it as an auth problem.
		if looksLikeHTML(body) {
			return nil, &AuthError{Status: resp.StatusCode, Body: "received an HTML sign-in page instead of JSON"}
		}
		return nil, fmt.Errorf("decoding audit response: %w", err)
	}
	if len(res.DecoratedAuditLogEntries) == 0 {
		var env queryEnvelope
		if json.Unmarshal(body, &env) == nil && len(env.Value.DecoratedAuditLogEntries) > 0 {
			return &env.Value, nil
		}
	}
	return &res, nil
}

// QueryAll walks the continuation tokens and accumulates every entry in
// the window, stopping at maxEntries (0 = no cap).
func (c *Client) QueryAll(ctx context.Context, opts QueryOptions, maxEntries int) ([]AuditEntry, error) {
	var all []AuditEntry
	token := opts.ContinuationToken
	seen := map[string]bool{}

	for {
		opts.ContinuationToken = token
		res, err := c.Query(ctx, opts)
		if err != nil {
			return all, err
		}
		all = append(all, res.DecoratedAuditLogEntries...)

		if maxEntries > 0 && len(all) >= maxEntries {
			return all[:maxEntries], nil
		}
		if !res.HasMore || res.ContinuationToken == "" {
			return all, nil
		}
		if seen[res.ContinuationToken] {
			return all, nil
		}
		seen[res.ContinuationToken] = true
		token = res.ContinuationToken
	}
}

// AuthError is returned when the org rejects the supplied credential. The
// server turns it into a helpful HTTP response.
type AuthError struct {
	Status   int
	Location string
	Body     string
}

func (e *AuthError) Error() string {
	hint := "authentication was rejected by Azure DevOps"
	if e.Location != "" && strings.Contains(strings.ToLower(e.Location), "signin") {
		hint = "PAT/token rejected — the organization redirected to interactive sign-in. " +
			"This org likely blocks PAT auth by policy; use Entra auth (azdo-ward connect <org> --entra)"
	} else if e.Status == http.StatusFound || e.Status == http.StatusNonAuthoritativeInfo {
		hint = "credential rejected — the organization forced an interactive sign-in. " +
			"If PAT auth is blocked by policy, use Entra auth (azdo-ward connect <org> --entra)"
	} else if e.Status == http.StatusForbidden {
		hint = "access forbidden — the identity lacks 'View audit log' permission (needs Project Collection Administrator)"
	} else if e.Status == http.StatusUnauthorized {
		hint = "unauthorized — the PAT is invalid/expired or missing the Audit Log (Read) scope"
	}
	if e.Body != "" {
		return fmt.Sprintf("%s (HTTP %d: %s)", hint, e.Status, e.Body)
	}
	return fmt.Sprintf("%s (HTTP %d)", hint, e.Status)
}

func isSignInRedirect(resp *http.Response) bool {
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return false
	}
	return strings.Contains(strings.ToLower(resp.Header.Get("Location")), "signin")
}

func looksLikeHTML(b []byte) bool {
	s := strings.TrimSpace(strings.ToLower(string(b)))
	return strings.HasPrefix(s, "<!doctype html") || strings.HasPrefix(s, "<html")
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
