package api

// QueryResult mirrors the AuditLogQueryResult object returned by the
// Azure DevOps Audit "Query" endpoint:
//
//	GET https://auditservice.dev.azure.com/{org}/_apis/audit/auditlog
//
// The real payload nests the result under a top-level "value" object, so
// we unwrap it in the client and hand callers the inner shape directly.
type QueryResult struct {
	DecoratedAuditLogEntries []AuditEntry `json:"decoratedAuditLogEntries"`
	ContinuationToken        string       `json:"continuationToken"`
	HasMore                  bool         `json:"hasMore"`
}

// queryEnvelope is the on-the-wire shape: { "value": { ... } }.
type queryEnvelope struct {
	Value QueryResult `json:"value"`
}

// AuditEntry is one DecoratedAuditLogEntry. Field names follow the REST
// schema exactly so json.Unmarshal works without tags surprises. The
// Data field is intentionally left as a free-form map because its shape
// depends on the ActionID (e.g. Project.CreateCompleted vs Git.RepositoryRenamed).
type AuditEntry struct {
	ID                      string                 `json:"id"`
	CorrelationID           string                 `json:"correlationId"`
	ActivityID              string                 `json:"activityId"`
	ActorCUID               string                 `json:"actorCUID"`
	ActorUserID             string                 `json:"actorUserId"`
	ActorClientID           string                 `json:"actorClientId"`
	ActorUPN                string                 `json:"actorUPN"`
	ActorDisplayName        string                 `json:"actorDisplayName"`
	ActorImageURL           string                 `json:"actorImageUrl"`
	AuthenticationMechanism string                 `json:"authenticationMechanism"`
	Timestamp               string                 `json:"timestamp"`
	ScopeType               string                 `json:"scopeType"`
	ScopeDisplayName        string                 `json:"scopeDisplayName"`
	ScopeID                 string                 `json:"scopeId"`
	ProjectID               string                 `json:"projectId"`
	ProjectName             string                 `json:"projectName"`
	IPAddress               string                 `json:"ipAddress"`
	UserAgent               string                 `json:"userAgent"`
	ActionID                string                 `json:"actionId"`
	Area                    string                 `json:"area"`
	Category                string                 `json:"category"`
	CategoryDisplayName     string                 `json:"categoryDisplayName"`
	Details                 string                 `json:"details"`
	Data                    map[string]interface{} `json:"data"`
}
