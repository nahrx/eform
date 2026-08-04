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
	// internal saja, tidak diserialisasi
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

// ViewerFormPermission menyimpan hak akses seorang viewer ke satu kuesioner.
type ViewerFormPermission struct {
	ID               string            `json:"id"`
	ViewerID         string            `json:"viewerId"`
	FormID           string            `json:"formId"`
	RespondentAccess string            `json:"respondentAccess"` // 'all' | 'selected'
	VisibleFields    []string          `json:"visibleFields"`    // nil = semua field
	FieldFilters     map[string]string `json:"fieldFilters"`     // fieldName → nilai wajib (exact match)
	CreatedBy        *string           `json:"createdBy,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	// Diisi saat listing (join)
	ViewerUsername string `json:"viewerUsername,omitempty"`
	FormTitle      string `json:"formTitle,omitempty"`
	AllowedCount   int    `json:"allowedCount,omitempty"`
}

// ViewerAllowedRespondent adalah satu responden yang diizinkan dalam mode 'selected'.
type ViewerAllowedRespondent struct {
	ID           string    `json:"id"`
	PermissionID string    `json:"permissionId"`
	RespondentID string    `json:"respondentId"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// EditorFormPermission menyimpan hak kelola seorang editor ke satu kuesioner.
type EditorFormPermission struct {
	ID               string            `json:"id"`
	EditorID         string            `json:"editorId"`
	FormID           string            `json:"formId"`
	RespondentAccess string            `json:"respondentAccess"` // 'all' | 'selected'
	FieldFilters     map[string]string `json:"fieldFilters"`     // fieldName → nilai wajib (exact match)
	CreatedBy        *string           `json:"createdBy,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	EditorName       string            `json:"editorName,omitempty"`
	FormTitle        string            `json:"formTitle,omitempty"`
	AllowedCount     int               `json:"allowedCount,omitempty"`
}

// EditorAllowedRespondent adalah satu responden yang diizinkan dalam mode 'selected'.
type EditorAllowedRespondent struct {
	ID           string    `json:"id"`
	PermissionID string    `json:"permissionId"`
	RespondentID string    `json:"respondentId"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// FormAPIKey adalah kredensial yang dipakai sistem eksternal untuk menarik jawaban
// satu kuesioner lewat /api/v1. Cakupan datanya mengikuti model ViewerFormPermission.
//
// Key aslinya tidak pernah disimpan: yang ada di DB hanya SHA-256 hex-nya (KeyHash,
// tidak diserialisasi) dan KeyPrefix untuk identifikasi. Key plaintext hanya
// dikembalikan sekali oleh handler pembuatan/rotasi.
type FormAPIKey struct {
	ID                string            `json:"id"`
	FormID            string            `json:"formId"`
	Label             string            `json:"label"`
	KeyPrefix         string            `json:"keyPrefix"`
	RespondentAccess  string            `json:"respondentAccess"` // 'all' | 'selected'
	VisibleFields     []string          `json:"visibleFields"`    // nil = semua variabel
	FieldFilters      map[string]string `json:"fieldFilters"`     // fieldName → nilai wajib (exact match)
	IncludeRespondent bool              `json:"includeRespondent"`
	AllowedIPs        []string          `json:"allowedIps"` // kosong = semua IP; isi = IP atau CIDR
	RateLimitPerMin   int               `json:"rateLimitPerMin"`
	IsActive          bool              `json:"isActive"`
	ExpiresAt         *time.Time        `json:"expiresAt,omitempty"`
	LastUsedAt        *time.Time        `json:"lastUsedAt,omitempty"`
	LastUsedIP        string            `json:"lastUsedIp,omitempty"`
	RequestCount      int64             `json:"requestCount"`
	CreatedBy         *string           `json:"createdBy,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
	// Diisi saat listing (join)
	AllowedCount int `json:"allowedCount,omitempty"`
	// internal saja, tidak diserialisasi
	KeyHash string `json:"-"`
}

// APIKeyAllowedRespondent adalah satu responden yang diizinkan dalam mode 'selected'.
type APIKeyAllowedRespondent struct {
	ID           string    `json:"id"`
	PermissionID string    `json:"permissionId"`
	RespondentID string    `json:"respondentId"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// APIAccessLog mencatat satu panggilan ke /api/v1, termasuk yang ditolak.
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
