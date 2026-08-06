package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email,omitempty"`
	PasswordHash      string    `json:"-"`
	Note              string    `json:"note,omitempty"`
	Role              string    `json:"role"`
	IsActive          bool      `json:"isActive"`
	TokenVersion      int       `json:"-"`                 // bumped to revoke existing sessions
	PreferredLanguage string    `json:"preferredLanguage"` // 'id' | 'en' — bahasa UI builder/dashboard
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Form struct {
	ID           string          `json:"id"`
	Slug         string          `json:"slug"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Schema       json.RawMessage `json:"schema,omitempty"`
	Status       string          `json:"status"`
	Version      string          `json:"version"`
	OwnerID      *string         `json:"ownerId,omitempty"`
	ColumnConfig json.RawMessage `json:"columnConfig,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	// ResponseCount is filled during listing (the number of submitted responses), so the
	// dashboard does not have to call a count endpoint per row.
	ResponseCount int64 `json:"responseCount"`
}

type Share struct {
	ID             string     `json:"id"`
	FormID         string     `json:"formId"`
	Token          string     `json:"token"`
	Label          string     `json:"label"`
	IsActive       bool       `json:"isActive"`
	AllowResponses bool       `json:"allowResponses"`
	MultiResponse  bool       `json:"multiResponse"`
	AccessMode     string     `json:"accessMode"`
	HasPassword    bool       `json:"hasPassword"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	ViewCount      int64      `json:"viewCount"`
	CreatedAt      time.Time  `json:"createdAt"`
	// internal only, never serialised
	PasswordHash *string `json:"-"`
}

type ShareAllowedEmail struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"shareId"`
	Email     string    `json:"email"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

type Response struct {
	ID           string          `json:"id"`
	FormID       string          `json:"formId"`
	ShareID      *string         `json:"shareId,omitempty"`
	RespondentID *string         `json:"respondentId,omitempty"`
	Status       string          `json:"status"` // 'submitted' | 'draft'
	Answers      json.RawMessage `json:"answers"`
	Meta         json.RawMessage `json:"meta,omitempty"`
	SubmittedAt  time.Time       `json:"submittedAt"`
}

type Draft struct {
	ID           string          `json:"id"`
	FormID       string          `json:"formId"`
	ShareID      *string         `json:"shareId,omitempty"`
	RespondentID string          `json:"respondentId"`
	Answers      json.RawMessage `json:"answers"`
	CurPage      int             `json:"curPage"`
	SavedAt      time.Time       `json:"savedAt"`
}

type WilayahItem struct {
	KodeWilayah string `json:"kode_wilayah"`
	NamaWilayah string `json:"nama_wilayah"`
}

type Respondent struct {
	ID        string    `json:"id"`
	GoogleID  string    `json:"googleId"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ViewerFormPermission holds one viewer's access rights to a single form.
type ViewerFormPermission struct {
	ID               string            `json:"id"`
	ViewerID         string            `json:"viewerId"`
	FormID           string            `json:"formId"`
	RespondentAccess string            `json:"respondentAccess"` // 'all' | 'selected'
	VisibleFields    []string          `json:"visibleFields"`    // nil = every field
	FieldFilters     map[string]string `json:"fieldFilters"`     // fieldName → required value (exact match)
	CreatedBy        *string           `json:"createdBy,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	// filled during listing (join)
	ViewerUsername string `json:"viewerUsername,omitempty"`
	FormTitle      string `json:"formTitle,omitempty"`
	AllowedCount   int    `json:"allowedCount,omitempty"`
}

// ViewerAllowedRespondent is a single respondent allowed under 'selected' mode.
type ViewerAllowedRespondent struct {
	ID           string    `json:"id"`
	PermissionID string    `json:"permissionId"`
	RespondentID string    `json:"respondentId"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// EditorFormPermission holds one editor's management rights over a single form.
type EditorFormPermission struct {
	ID               string            `json:"id"`
	EditorID         string            `json:"editorId"`
	FormID           string            `json:"formId"`
	RespondentAccess string            `json:"respondentAccess"` // 'all' | 'selected'
	FieldFilters     map[string]string `json:"fieldFilters"`     // fieldName → required value (exact match)
	CreatedBy        *string           `json:"createdBy,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	EditorName       string            `json:"editorName,omitempty"`
	FormTitle        string            `json:"formTitle,omitempty"`
	AllowedCount     int               `json:"allowedCount,omitempty"`
}

// EditorAllowedRespondent is a single respondent allowed under 'selected' mode.
type EditorAllowedRespondent struct {
	ID           string    `json:"id"`
	PermissionID string    `json:"permissionId"`
	RespondentID string    `json:"respondentId"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// FormAPIKey is the credential an external system uses to pull one form's responses
// through /api/v1. Its data scope follows the ViewerFormPermission model.
//
// The real key is never stored: the database holds only its SHA-256 hex digest
// (KeyHash, not serialised) and KeyPrefix for identification. The plaintext key is
// returned exactly once, by the create/rotate handler.
type FormAPIKey struct {
	ID                string            `json:"id"`
	FormID            string            `json:"formId"`
	Label             string            `json:"label"`
	KeyPrefix         string            `json:"keyPrefix"`
	RespondentAccess  string            `json:"respondentAccess"` // 'all' | 'selected'
	VisibleFields     []string          `json:"visibleFields"`    // nil = every field
	FieldFilters      map[string]string `json:"fieldFilters"`     // fieldName → required value (exact match)
	IncludeRespondent bool              `json:"includeRespondent"`
	AllowedIPs        []string          `json:"allowedIps"` // empty = any IP; set = IP or CIDR
	RateLimitPerMin   int               `json:"rateLimitPerMin"`
	IsActive          bool              `json:"isActive"`
	ExpiresAt         *time.Time        `json:"expiresAt,omitempty"`
	LastUsedAt        *time.Time        `json:"lastUsedAt,omitempty"`
	LastUsedIP        string            `json:"lastUsedIp,omitempty"`
	RequestCount      int64             `json:"requestCount"`
	CreatedBy         *string           `json:"createdBy,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
	// filled during listing (join)
	AllowedCount int `json:"allowedCount,omitempty"`
	// internal only, never serialised
	KeyHash string `json:"-"`
}

// APIKeyAllowedRespondent is a single respondent allowed under 'selected' mode.
type APIKeyAllowedRespondent struct {
	ID           string    `json:"id"`
	PermissionID string    `json:"permissionId"`
	RespondentID string    `json:"respondentId"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ResponseRevision records one answer change made by an editor, including the before
// and after values, so data corrections can be traced.
type ResponseRevision struct {
	ID            string          `json:"id"`
	ResponseID    string          `json:"responseId"`
	FormID        string          `json:"formId"`
	EditorID      *string         `json:"editorId,omitempty"`
	EditorName    string          `json:"editorName"`
	AnswersBefore json.RawMessage `json:"answersBefore"`
	AnswersAfter  json.RawMessage `json:"answersAfter"`
	IP            string          `json:"ip"`
	CreatedAt     time.Time       `json:"createdAt"`
	// Filled during listing: the names of the changed fields, so the UI does not have to diff them itself.
	ChangedFields []string `json:"changedFields,omitempty"`
}

// OfflineQueueReport is one device's latest account of what its offline queue is
// still holding. Metadata only: it says that work is stranded and on whose phone,
// not what the work was. Recovering the answers still means reaching the user.
type OfflineQueueReport struct {
	ID           string `json:"id"`
	FormID       string `json:"formId"`
	RespondentID string `json:"respondentId"`
	DeviceID     string `json:"deviceId"`

	Pending int `json:"pending"`
	Failed  int `json:"failed"`
	Files   int `json:"files"`

	OldestQueuedAt *time.Time      `json:"oldestQueuedAt,omitempty"`
	Items          json.RawMessage `json:"items"`
	UserAgent      string          `json:"userAgent"`
	ReportedAt     time.Time       `json:"reportedAt"`

	// Joined during listing so an admin can actually contact the user — the whole
	// point of the report.
	RespondentName  string `json:"respondentName,omitempty"`
	RespondentEmail string `json:"respondentEmail,omitempty"`
}

// ActivityLog records one admin/superadmin action that changes data or takes data
// out of the system (e.g. a CSV export).
type ActivityLog struct {
	ID          string    `json:"id"`
	ActorID     *string   `json:"actorId,omitempty"`
	ActorName   string    `json:"actorName"`
	ActorRole   string    `json:"actorRole"`
	Action      string    `json:"action"`
	TargetType  string    `json:"targetType,omitempty"`
	TargetID    string    `json:"targetId,omitempty"`
	TargetLabel string    `json:"targetLabel,omitempty"`
	FormID      *string   `json:"formId,omitempty"`
	IP          string    `json:"ip"`
	Detail      string    `json:"detail,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// APIAccessLog records one call to /api/v1, including rejected ones.
type APIAccessLog struct {
	ID        string    `json:"id"`
	APIKeyID  *string   `json:"apiKeyId,omitempty"`
	KeyPrefix string    `json:"keyPrefix"`
	FormID    *string   `json:"formId,omitempty"`
	IP        string    `json:"ip"`
	Path      string    `json:"path"`
	Query     string    `json:"query,omitempty"`
	Status    int       `json:"status"`
	RowCount  int       `json:"rowCount"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
