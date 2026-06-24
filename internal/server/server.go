// Package server wires the embedded SPA to the Audit API. The PAT never
// leaves the machine: the browser talks to this local server, which holds
// the token in config and proxies queries to Azure DevOps.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/dutraph/azdo-ward/internal/api"
	"github.com/dutraph/azdo-ward/internal/auth"
	"github.com/dutraph/azdo-ward/internal/config"
	"github.com/dutraph/azdo-ward/internal/version"
	"github.com/dutraph/azdo-ward/internal/web"
)

// clientFor builds an Audit API client for an org using its auth mode.
func clientFor(o *config.Org) (*api.Client, error) {
	switch o.Mode() {
	case config.AuthEntra:
		return api.New(o.Name, api.BearerAuth(auth.EntraToken)), nil
	default:
		if o.PAT == "" {
			return nil, errNoCredential
		}
		return api.New(o.Name, api.PATAuth(o.PAT)), nil
	}
}

var errNoCredential = &credErr{}

type credErr struct{}

func (*credErr) Error() string { return "no credential configured for this organization" }

// Server is the HTTP application.
type Server struct {
	cfg *config.Config
	mux *http.ServeMux
}

// New builds a Server backed by the given config.
func New(cfg *config.Config) (*Server, error) {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}

	sub, err := fs.Sub(web.Static, "static")
	if err != nil {
		return nil, err
	}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))

	s.mux.HandleFunc("/api/state", s.handleState)
	s.mux.HandleFunc("/api/connect", s.handleConnect)
	s.mux.HandleFunc("/api/switch", s.handleSwitch)
	s.mux.HandleFunc("/api/remove", s.handleRemove)
	s.mux.HandleFunc("/api/audit", s.handleAudit)
	return s, nil
}

// Handler exposes the mux (handy for tests).
func (s *Server) Handler() http.Handler { return s.mux }

// Listen starts the blocking HTTP server on addr.
func (s *Server) Listen(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// handleState reports which orgs are configured and which is active so the
// UI can decide whether to show the connect screen or the dashboard.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	type orgInfo struct {
		Name string `json:"name"`
		Auth string `json:"auth"`
	}
	orgs := make([]orgInfo, 0, len(s.cfg.Orgs))
	for i := range s.cfg.Orgs {
		orgs = append(orgs, orgInfo{Name: s.cfg.Orgs[i].Name, Auth: s.cfg.Orgs[i].Mode()})
	}
	active := s.cfg.ActiveOrg()
	activeAuth := ""
	if active != nil {
		activeAuth = active.Mode()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":    version.String(),
		"orgs":       orgs,
		"active":     s.cfg.Active,
		"activeAuth": activeAuth,
		"configured": active != nil,
	})
}

type connectReq struct {
	Org  string `json:"org"`
	PAT  string `json:"pat"`
	Auth string `json:"auth"` // "pat" (default) or "entra"
}

// handleConnect saves (or switches to) an org. For PAT auth a token is
// required on first connect; for Entra auth no secret is stored. A known
// org name with blank fields just switches the active account.
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req connectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Org == "" {
		writeErr(w, http.StatusBadRequest, "org is required")
		return
	}
	if req.Auth != "" && req.Auth != config.AuthPAT && req.Auth != config.AuthEntra {
		writeErr(w, http.StatusBadRequest, "auth must be 'pat' or 'entra'")
		return
	}
	s.cfg.Upsert(req.Org, req.PAT, req.Auth)
	if err := s.cfg.Save(); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save config: "+err.Error())
		return
	}
	s.handleState(w, r)
}

type orgReq struct {
	Org string `json:"org"`
}

// handleSwitch changes the active org to an already-configured one (no PAT
// needed). The persisted config is the source of truth, so a switch sticks
// across restarts.
func (s *Server) handleSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req orgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Org == "" {
		writeErr(w, http.StatusBadRequest, "org is required")
		return
	}
	if !s.cfg.SetActive(req.Org) {
		writeErr(w, http.StatusNotFound, "unknown organization: "+req.Org)
		return
	}
	if err := s.cfg.Save(); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save config: "+err.Error())
		return
	}
	s.handleState(w, r)
}

// handleRemove deletes a saved org from the config.
func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req orgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Org == "" {
		writeErr(w, http.StatusBadRequest, "org is required")
		return
	}
	if !s.cfg.Remove(req.Org) {
		writeErr(w, http.StatusNotFound, "unknown organization: "+req.Org)
		return
	}
	if err := s.cfg.Save(); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save config: "+err.Error())
		return
	}
	s.handleState(w, r)
}

// handleAudit proxies a windowed query to the Audit API and returns the
// decorated entries verbatim for the frontend to render.
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	active := s.cfg.ActiveOrg()
	if active == nil {
		writeErr(w, http.StatusPreconditionRequired, "no organization configured — connect first")
		return
	}
	client, err := clientFor(active)
	if err != nil {
		writeErr(w, http.StatusPreconditionRequired, err.Error())
		return
	}

	q := r.URL.Query()
	opts := api.QueryOptions{BatchSize: 1000}
	if v := q.Get("start"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			opts.StartTime = t
		}
	}
	if v := q.Get("end"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			opts.EndTime = t
		}
	}
	// Default window: last 7 days when nothing is supplied.
	if opts.StartTime.IsZero() && opts.EndTime.IsZero() {
		opts.EndTime = time.Now().UTC()
		opts.StartTime = opts.EndTime.Add(-7 * 24 * time.Hour)
	}
	maxEntries := 5000
	if v := q.Get("max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxEntries = n
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	entries, err := client.QueryAll(ctx, opts, maxEntries)
	if err != nil {
		// Auth failures get a 401 so the UI can prompt re-connect; other
		// upstream issues are a 502.
		code := http.StatusBadGateway
		var ae *api.AuthError
		if errors.As(err, &ae) {
			code = http.StatusUnauthorized
		}
		writeErr(w, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"org":     active.Name,
		"count":   len(entries),
		"entries": entries,
	})
}
