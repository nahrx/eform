package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResponseFilter parameter untuk filter dan sort daftar jawaban admin.
type ResponseFilter struct {
	Status            string            // 'submitted'|'draft'|'' (kosong = semua)
	ShareID           string            // uuid string atau '' (kosong = semua)
	Search            string            // pencarian parsial pada meta.name / meta.email
	SortBy            string            // 'waktu'|'status'|'share'|'who'|nama_field_schema
	SortDir           string            // 'asc'|'desc'
	FieldFilters      map[string]string // fieldName → nilai teks (ILIKE, untuk field bebas)
	FieldExactFilters map[string]string // fieldName → nilai pasti (=, untuk dropdown/radio)
}

// isSafeIdentifier memvalidasi nama field schema agar aman diinterpolasi ke SQL.
// Hanya huruf, angka, dan underscore diizinkan.
func isSafeIdentifier(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// buildResponseWhere membangun klausa WHERE dan slice args untuk query daftar/count jawaban.
// args dimulai dari $2 (karena $1 selalu formID).
func buildResponseWhere(f ResponseFilter) (string, []any) {
	var args []any
	where := ""
	add := func(v any) int {
		args = append(args, v)
		return len(args) + 1 // +1 karena $1=formID sudah di luar
	}
	if f.Status != "" {
		n := add(f.Status)
		where += fmt.Sprintf(" AND status=$%d", n)
	}
	if f.ShareID != "" {
		n := add(f.ShareID)
		where += fmt.Sprintf(" AND share_id::text=$%d", n)
	}
	if f.Search != "" {
		n := add("%" + f.Search + "%")
		where += fmt.Sprintf(" AND (meta->>'name' ILIKE $%d OR meta->>'email' ILIKE $%d)", n, n)
	}
	for fieldName, val := range f.FieldFilters {
		if isSafeIdentifier(fieldName) && val != "" {
			n := add("%" + val + "%")
			where += fmt.Sprintf(" AND answers->>'%s' ILIKE $%d", fieldName, n)
		}
	}
	for fieldName, val := range f.FieldExactFilters {
		if isSafeIdentifier(fieldName) && val != "" {
			n := add(val)
			where += fmt.Sprintf(" AND answers->>'%s'=$%d", fieldName, n)
		}
	}
	return where, args
}

var ErrNotFound = errors.New("data tidak ditemukan")

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

/* ---------------- users ---------------- */

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, email, hash, role, note string) (*models.User, error) {
	var emailArg, noteArg any
	if email != "" {
		emailArg = email
	}
	if note != "" {
		noteArg = note
	}
	u := &models.User{}
	var em, nt *string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users(username,email,password_hash,role,note) VALUES ($1,$2,$3,$4,$5)
		 RETURNING id,username,email,role,note,is_active,created_at,updated_at`,
		username, emailArg, hash, role, noteArg,
	).Scan(&u.ID, &u.Username, &em, &u.Role, &nt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if em != nil {
		u.Email = *em
	}
	if nt != nil {
		u.Note = *nt
	}
	return u, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	u := &models.User{}
	var em *string
	err := s.pool.QueryRow(ctx,
		`SELECT id,username,email,password_hash,role,is_active,created_at,updated_at
		 FROM users WHERE username=$1`, username,
	).Scan(&u.ID, &u.Username, &em, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if em != nil {
		u.Email = *em
	}
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,username,email,role,is_active,created_at,updated_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u := models.User{}
		var em *string
		if err := rows.Scan(&u.ID, &u.Username, &em, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if em != nil {
			u.Email = *em
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

/* ---------------- forms ---------------- */

func (s *Store) CreateForm(ctx context.Context, slug, title, desc string, schema json.RawMessage, version string, ownerID *string) (*models.Form, error) {
	if len(schema) == 0 {
		schema = json.RawMessage(`{}`)
	}
	f := &models.Form{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO forms(slug,title,description,schema,version,owner_id)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id,slug,title,description,status,version,owner_id,created_at,updated_at`,
		slug, title, desc, schema, version, ownerID,
	).Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Status, &f.Version, &f.OwnerID, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Store) GetForm(ctx context.Context, id string) (*models.Form, error) {
	f := &models.Form{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,slug,title,description,schema,status,version,owner_id,created_at,updated_at
		 FROM forms WHERE id=$1`, id,
	).Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Schema, &f.Status, &f.Version, &f.OwnerID, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// ListForms tidak mengembalikan schema (hemat payload untuk daftar).
func (s *Store) ListForms(ctx context.Context) ([]models.Form, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,slug,title,description,status,version,owner_id,created_at,updated_at
		 FROM forms ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Form
	for rows.Next() {
		f := models.Form{}
		if err := rows.Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Status, &f.Version, &f.OwnerID, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ListFormsByOwner mengembalikan daftar form milik owner tertentu.
func (s *Store) ListFormsByOwner(ctx context.Context, ownerID string) ([]models.Form, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,slug,title,description,status,version,owner_id,created_at,updated_at
		 FROM forms WHERE owner_id=$1 ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Form
	for rows.Next() {
		f := models.Form{}
		if err := rows.Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Status, &f.Version, &f.OwnerID, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ListFormsByEditor mengembalikan daftar form yang ditugaskan ke editor.
func (s *Store) ListFormsByEditor(ctx context.Context, editorID string) ([]models.Form, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT f.id,f.slug,f.title,f.description,f.status,f.version,f.owner_id,f.created_at,f.updated_at
		 FROM forms f
		 JOIN editor_form_permissions p ON p.form_id=f.id
		 WHERE p.editor_id=$1
		 ORDER BY f.updated_at DESC`, editorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Form
	for rows.Next() {
		f := models.Form{}
		if err := rows.Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Status, &f.Version, &f.OwnerID, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) UpdateForm(ctx context.Context, id, title, desc string, schema json.RawMessage, version string) (*models.Form, error) {
	f := &models.Form{}
	err := s.pool.QueryRow(ctx,
		`UPDATE forms SET title=$2, description=$3, schema=$4, version=$5, updated_at=now()
		 WHERE id=$1
		 RETURNING id,slug,title,description,status,version,owner_id,created_at,updated_at`,
		id, title, desc, schema, version,
	).Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Status, &f.Version, &f.OwnerID, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

func (s *Store) SetFormStatus(ctx context.Context, id, status string) error {
	ct, err := s.pool.Exec(ctx, `UPDATE forms SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteForm(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM forms WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM forms WHERE slug=$1)`, slug).Scan(&exists)
	return exists, err
}

/* ---------------- shares ---------------- */

func (s *Store) CreateShare(ctx context.Context, formID, token, label string, allowResponses, multiResponse bool, accessMode string, passwordHash *string, expiresAt *time.Time, createdBy *string) (*models.Share, error) {
	if accessMode == "" {
		accessMode = "public"
	}
	sh := &models.Share{}
	var ph *string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO form_shares(form_id,token,label,allow_responses,multi_response,access_mode,password_hash,expires_at,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id,form_id,token,label,is_active,allow_responses,multi_response,access_mode,password_hash,expires_at,view_count,created_at`,
		formID, token, label, allowResponses, multiResponse, accessMode, passwordHash, expiresAt, createdBy,
	).Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &sh.MultiResponse, &sh.AccessMode, &ph, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt)
	if err != nil {
		return nil, err
	}
	sh.HasPassword = ph != nil
	return sh, nil
}

func (s *Store) ListSharesByForm(ctx context.Context, formID string) ([]models.Share, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,form_id,token,label,is_active,allow_responses,multi_response,access_mode,password_hash,expires_at,view_count,created_at
		 FROM form_shares WHERE form_id=$1 ORDER BY created_at DESC`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Share
	for rows.Next() {
		sh := models.Share{}
		var ph *string
		if err := rows.Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &sh.MultiResponse, &sh.AccessMode, &ph, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt); err != nil {
			return nil, err
		}
		sh.HasPassword = ph != nil
		out = append(out, sh)
	}
	return out, rows.Err()
}

func (s *Store) GetShareByToken(ctx context.Context, token string) (*models.Share, error) {
	sh := &models.Share{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,form_id,token,label,is_active,allow_responses,multi_response,access_mode,password_hash,expires_at,view_count,created_at
		 FROM form_shares WHERE token=$1`, token,
	).Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &sh.MultiResponse, &sh.AccessMode, &sh.PasswordHash, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sh.HasPassword = sh.PasswordHash != nil
	return sh, nil
}

// GetShareByID mengambil satu share berdasarkan ID.
func (s *Store) GetShareByID(ctx context.Context, id string) (*models.Share, error) {
	sh := &models.Share{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,form_id,token,label,is_active,allow_responses,multi_response,access_mode,password_hash,expires_at,view_count,created_at
		 FROM form_shares WHERE id=$1`, id,
	).Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &sh.MultiResponse, &sh.AccessMode, &sh.PasswordHash, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sh.HasPassword = sh.PasswordHash != nil
	return sh, nil
}

func (s *Store) RevokeShare(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `UPDATE form_shares SET is_active=false WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateShare memperbarui konfigurasi share yang masih aktif.
// updatePassword=true  → password_hash diubah ke passwordHash (nil berarti hapus password).
// updateExpiry=true    → expires_at diubah ke expiresAt (nil berarti hapus expiry).
func (s *Store) UpdateShare(ctx context.Context, id, label string, allowResponses, multiResponse bool, accessMode string, updatePassword bool, passwordHash *string, updateExpiry bool, expiresAt *time.Time) (*models.Share, error) {
	if accessMode != "public" && accessMode != "restricted" {
		accessMode = "public"
	}
	sh := &models.Share{}
	var ph *string
	err := s.pool.QueryRow(ctx, `
		UPDATE form_shares SET
		  label=$2,
		  allow_responses=$3,
		  multi_response=$4,
		  access_mode=$5,
		  password_hash = CASE WHEN $6 THEN $7 ELSE password_hash END,
		  expires_at    = CASE WHEN $8 THEN $9 ELSE expires_at END
		WHERE id=$1
		RETURNING id,form_id,token,label,is_active,allow_responses,multi_response,access_mode,password_hash,expires_at,view_count,created_at`,
		id, label, allowResponses, multiResponse, accessMode,
		updatePassword, passwordHash,
		updateExpiry, expiresAt,
	).Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &sh.MultiResponse, &sh.AccessMode, &ph, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sh.HasPassword = ph != nil
	return sh, nil
}

func (s *Store) IncrementShareView(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE form_shares SET view_count = view_count + 1 WHERE id=$1`, id)
}

// DeleteShare menghapus permanen share yang sudah dicabut (is_active=false).
func (s *Store) DeleteShare(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM form_shares WHERE id=$1 AND is_active=false`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

/* ---------------- share allowed emails ---------------- */

func (s *Store) CreateShareAllowedEmail(ctx context.Context, shareID, email, note string) (*models.ShareAllowedEmail, error) {
	e := &models.ShareAllowedEmail{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO share_allowed_emails(share_id,email,note) VALUES ($1,$2,$3)
		 ON CONFLICT (share_id,email) DO UPDATE SET note=EXCLUDED.note
		 RETURNING id,share_id,email,note,created_at`,
		shareID, email, note,
	).Scan(&e.ID, &e.ShareID, &e.Email, &e.Note, &e.CreatedAt)
	return e, err
}

func (s *Store) ListShareAllowedEmails(ctx context.Context, shareID string) ([]models.ShareAllowedEmail, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,share_id,email,note,created_at FROM share_allowed_emails WHERE share_id=$1 ORDER BY created_at`, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ShareAllowedEmail
	for rows.Next() {
		e := models.ShareAllowedEmail{}
		if err := rows.Scan(&e.ID, &e.ShareID, &e.Email, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetShareAllowedEmailByID mengambil data allowlist email per ID.
func (s *Store) GetShareAllowedEmailByID(ctx context.Context, id string) (*models.ShareAllowedEmail, error) {
	e := &models.ShareAllowedEmail{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,share_id,email,note,created_at FROM share_allowed_emails WHERE id=$1`, id,
	).Scan(&e.ID, &e.ShareID, &e.Email, &e.Note, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Store) DeleteShareAllowedEmail(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM share_allowed_emails WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) IsEmailAllowed(ctx context.Context, shareID, email string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM share_allowed_emails WHERE share_id=$1 AND lower(email)=lower($2))`,
		shareID, email,
	).Scan(&exists)
	return exists, err
}

/* ---------------- responses ---------------- */

func (s *Store) CreateResponse(ctx context.Context, formID string, shareID *string, answers, meta json.RawMessage) (*models.Response, error) {
	if len(answers) == 0 {
		answers = json.RawMessage(`{}`)
	}
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO form_responses(form_id,share_id,answers,meta,status) VALUES ($1,$2,$3,$4,'submitted')
		 RETURNING id,form_id,share_id,respondent_id,status,answers,meta,submitted_at`,
		formID, shareID, answers, meta,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	return r, err
}

// CreateMultiResponseRow menyimpan baris baru di form_responses dengan status tertentu ('submitted' atau 'draft').
func (s *Store) CreateMultiResponseRow(ctx context.Context, formID string, shareID *string, respondentID, status string, answers, meta json.RawMessage) (*models.Response, error) {
	if len(answers) == 0 {
		answers = json.RawMessage(`{}`)
	}
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO form_responses(form_id,share_id,respondent_id,status,answers,meta) VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id,form_id,share_id,respondent_id,status,answers,meta,submitted_at`,
		formID, shareID, respondentID, status, answers, meta,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	return r, err
}

// GetResponseByID mengambil satu respons berdasarkan ID-nya.
// Jika tidak ditemukan di form_responses, cari di response_drafts (draf single-response).
func (s *Store) GetResponseByID(ctx context.Context, id string) (*models.Response, error) {
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		 FROM form_responses WHERE id=$1`, id,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	// Fallback: cari di response_drafts (draf form single-response)
	err2 := s.pool.QueryRow(ctx,
		`SELECT id,form_id,share_id,respondent_id,answers,'{}'::jsonb,saved_at
		 FROM response_drafts WHERE id=$1`, id,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Answers, &r.Meta, &r.SubmittedAt)
	if errors.Is(err2, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err2 != nil {
		return nil, err2
	}
	r.Status = "draft"
	return r, nil
}

// ListAllResponsesByForm mengembalikan semua respons (form_responses + response_drafts) untuk tampilan admin.
// Mendukung filter status/shareId/search/per-field dan sort dinamis termasuk field schema.
func (s *Store) ListAllResponsesByForm(ctx context.Context, formID string, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	// Sort column: cek fixed allowlist dulu, lalu coba sebagai nama field schema
	sortDir := "DESC"
	if f.SortDir == "asc" {
		sortDir = "ASC"
	}
	sortCol := map[string]string{
		"waktu":  "submitted_at",
		"status": "status",
		"share":  "share_id",
		"who":    "meta->>'name'",
	}[f.SortBy]
	if sortCol == "" && isSafeIdentifier(f.SortBy) {
		sortCol = fmt.Sprintf("answers->>'%s'", f.SortBy)
	}
	if sortCol == "" {
		sortCol = "submitted_at"
	}

	where, wArgs := buildResponseWhere(f)
	// args: $1=formID, lalu wArgs sebagai $2…$N, lalu limit=$N+1, offset=$N+2
	args := append([]any{formID}, wArgs...)
	args = append(args, limit, offset)
	limitN, offsetN := len(args)-1, len(args)

	q := fmt.Sprintf(`
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		    FROM form_responses WHERE form_id=$1
		  UNION ALL
		  SELECT id,form_id,share_id,respondent_id,'draft'::text,answers,'{}'::jsonb,saved_at
		    FROM response_drafts WHERE form_id=$1
		) combined
		WHERE 1=1%s
		ORDER BY %s %s NULLS LAST
		LIMIT $%d OFFSET $%d`, where, sortCol, sortDir, limitN, offsetN)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Response
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountAllResponsesByForm menghitung semua respons (semua status + response_drafts) sesuai filter.
func (s *Store) CountAllResponsesByForm(ctx context.Context, formID string, f ResponseFilter) (int64, error) {
	where, wArgs := buildResponseWhere(f)
	args := append([]any{formID}, wArgs...)
	var n int64
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*) FROM (
		  SELECT status,share_id,meta,answers FROM form_responses WHERE form_id=$1
		  UNION ALL
		  SELECT 'draft'::text,share_id,'{}'::jsonb,answers FROM response_drafts WHERE form_id=$1
		) combined
		WHERE 1=1%s`, where),
		args...,
	).Scan(&n)
	return n, err
}

// HasDraftResponse mengembalikan true jika responden masih memiliki draf aktif untuk form ini.
func (s *Store) HasDraftResponse(ctx context.Context, formID, respondentID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM form_responses WHERE form_id=$1 AND respondent_id=$2 AND status='draft')`,
		formID, respondentID,
	).Scan(&exists)
	return exists, err
}

// UpdateMultiResponseDraft memperbarui jawaban draf yang ada.
// newStatus='draft'     → update jawaban saja.
// newStatus='submitted' → update jawaban + submitted_at=now().
// Hanya berhasil jika baris masih berstatus 'draft' dan milik respondentID pada formID yang sama.
func (s *Store) UpdateMultiResponseDraft(ctx context.Context, id, respondentID, formID, newStatus string, answers, meta json.RawMessage) (*models.Response, error) {
	if len(answers) == 0 {
		answers = json.RawMessage(`{}`)
	}
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	r := &models.Response{}
	err := s.pool.QueryRow(ctx, `
		UPDATE form_responses SET
		  answers=$5, meta=$6, status=$4,
		  submitted_at = CASE WHEN $4='submitted' THEN now() ELSE submitted_at END
		WHERE id=$1 AND respondent_id=$2 AND form_id=$3 AND status='draft'
		RETURNING id,form_id,share_id,respondent_id,status,answers,meta,submitted_at`,
		id, respondentID, formID, newStatus, answers, meta,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// UnsubmitResponse mengubah status respons dari 'submitted' kembali ke 'draft' sehingga bisa diedit.
// Gagal jika sudah ada draf lain dari responden yang sama untuk form yang sama.
func (s *Store) UnsubmitResponse(ctx context.Context, id, respondentID, formID string) (*models.Response, error) {
	var draftExists bool
	_ = s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM form_responses WHERE form_id=$1 AND respondent_id=$2 AND status='draft' AND id!=$3)`,
		formID, respondentID, id,
	).Scan(&draftExists)
	if draftExists {
		return nil, errors.New("sudah ada draf lain — selesaikan atau batalkan draf tersebut terlebih dahulu")
	}
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`UPDATE form_responses SET status='draft'
		 WHERE id=$1 AND respondent_id=$2 AND form_id=$3 AND status='submitted'
		 RETURNING id,form_id,share_id,respondent_id,status,answers,meta,submitted_at`,
		id, respondentID, formID,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// ListResponsesByFormAndRespondent mengembalikan semua jawaban responden untuk form ini (multi-response).
func (s *Store) ListResponsesByFormAndRespondent(ctx context.Context, formID, respondentID string) ([]models.Response, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		 FROM form_responses WHERE form_id=$1 AND respondent_id=$2
		 ORDER BY submitted_at DESC`,
		formID, respondentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Response
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertResponse menyimpan jawaban yang terikat ke respondent.
// Jika sudah ada jawaban untuk (form_id, respondent_id), maka di-update.
func (s *Store) UpsertResponse(ctx context.Context, formID string, shareID *string, respondentID string, answers, meta json.RawMessage) (*models.Response, error) {
	if len(answers) == 0 {
		answers = json.RawMessage(`{}`)
	}
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO form_responses(form_id,share_id,respondent_id,status,answers,meta)
		 VALUES ($1,$2,$3,'submitted',$4,$5)
		 ON CONFLICT (form_id,respondent_id) WHERE respondent_id IS NOT NULL
		 DO UPDATE SET share_id=EXCLUDED.share_id, status='submitted', answers=EXCLUDED.answers,
		               meta=EXCLUDED.meta, submitted_at=now()
		 RETURNING id,form_id,share_id,respondent_id,status,answers,meta,submitted_at`,
		formID, shareID, respondentID, answers, meta,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	return r, err
}

// GetResponseByFormAndRespondent mengembalikan jawaban yang pernah dikirim responden untuk form ini.
func (s *Store) GetResponseByFormAndRespondent(ctx context.Context, formID, respondentID string) (*models.Response, error) {
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		 FROM form_responses WHERE form_id=$1 AND respondent_id=$2 AND status='submitted'`,
		formID, respondentID,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// UpsertRespondent membuat atau memperbarui data responden berdasarkan google_id.
func (s *Store) UpsertRespondent(ctx context.Context, googleID, email, name, picture string) (*models.Respondent, error) {
	r := &models.Respondent{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO respondents(google_id,email,name,picture) VALUES ($1,$2,$3,$4)
		 ON CONFLICT (google_id) DO UPDATE
		     SET email=EXCLUDED.email, name=EXCLUDED.name, picture=EXCLUDED.picture, updated_at=now()
		 RETURNING id,google_id,email,name,picture,created_at,updated_at`,
		googleID, email, name, picture,
	).Scan(&r.ID, &r.GoogleID, &r.Email, &r.Name, &r.Picture, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func (s *Store) ListResponsesByForm(ctx context.Context, formID string, limit, offset int) ([]models.Response, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		 FROM form_responses WHERE form_id=$1 AND status='submitted' ORDER BY submitted_at DESC LIMIT $2 OFFSET $3`,
		formID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Response
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CountResponsesByForm(ctx context.Context, formID string) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM form_responses WHERE form_id=$1 AND status='submitted'`, formID).Scan(&n)
	return n, err
}

/* ---------------- drafts ---------------- */

func (s *Store) UpsertDraft(ctx context.Context, formID string, shareID *string, respondentID string, answers json.RawMessage, curPage int) (*models.Draft, error) {
	if len(answers) == 0 {
		answers = json.RawMessage(`{}`)
	}
	d := &models.Draft{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO response_drafts(form_id,share_id,respondent_id,answers,cur_page)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (form_id,respondent_id)
		 DO UPDATE SET share_id=EXCLUDED.share_id, answers=EXCLUDED.answers,
		               cur_page=EXCLUDED.cur_page, saved_at=now()
		 RETURNING id,form_id,share_id,respondent_id,answers,cur_page,saved_at`,
		formID, shareID, respondentID, answers, curPage,
	).Scan(&d.ID, &d.FormID, &d.ShareID, &d.RespondentID, &d.Answers, &d.CurPage, &d.SavedAt)
	return d, err
}

func (s *Store) GetDraftByFormAndRespondent(ctx context.Context, formID, respondentID string) (*models.Draft, error) {
	d := &models.Draft{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,form_id,share_id,respondent_id,answers,cur_page,saved_at
		 FROM response_drafts WHERE form_id=$1 AND respondent_id=$2`,
		formID, respondentID,
	).Scan(&d.ID, &d.FormID, &d.ShareID, &d.RespondentID, &d.Answers, &d.CurPage, &d.SavedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

func (s *Store) DeleteDraft(ctx context.Context, formID, respondentID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM response_drafts WHERE form_id=$1 AND respondent_id=$2`,
		formID, respondentID)
	return err
}

// GetUserByEmail mencari user berdasarkan alamat email (dipakai untuk Google OAuth viewer).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	var em *string
	err := s.pool.QueryRow(ctx,
		`SELECT id,username,email,password_hash,role,is_active,created_at,updated_at
		 FROM users WHERE lower(email)=lower($1)`, email,
	).Scan(&u.ID, &u.Username, &em, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if em != nil {
		u.Email = *em
	}
	return u, nil
}

/* ---------------- viewers ---------------- */

// ListViewers mengembalikan semua user dengan role='viewer'.
func (s *Store) ListViewers(ctx context.Context) ([]models.User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,username,email,role,note,is_active,created_at,updated_at FROM users
		 WHERE role='viewer' ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u := models.User{}
		var em, nt *string
		if err := rows.Scan(&u.ID, &u.Username, &em, &u.Role, &nt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if em != nil {
			u.Email = *em
		}
		if nt != nil {
			u.Note = *nt
		}
		out = append(out, u)
	}
	if out == nil {
		out = []models.User{}
	}
	return out, rows.Err()
}

// ListEditors mengembalikan semua user dengan role='editor'.
func (s *Store) ListEditors(ctx context.Context) ([]models.User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,username,email,role,note,is_active,created_at,updated_at FROM users
		 WHERE role='editor' ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u := models.User{}
		var em, nt *string
		if err := rows.Scan(&u.ID, &u.Username, &em, &u.Role, &nt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if em != nil {
			u.Email = *em
		}
		if nt != nil {
			u.Note = *nt
		}
		out = append(out, u)
	}
	if out == nil {
		out = []models.User{}
	}
	return out, rows.Err()
}

// DeleteUser menghapus user secara permanen (dimaksudkan untuk viewer).
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateViewerPermission memberikan akses viewer ke satu kuesioner.
func (s *Store) CreateViewerPermission(ctx context.Context, viewerID, formID, respondentAccess string, visibleFields []string, createdBy *string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO viewer_form_permissions(viewer_id,form_id,respondent_access,visible_fields,created_by)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at`,
		viewerID, formID, respondentAccess, visibleFields, createdBy,
	).Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt)
	return p, err
}

// GetViewerPermission mengambil permission viewer-form, atau ErrNotFound.
func (s *Store) GetViewerPermission(ctx context.Context, viewerID, formID string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at
		 FROM viewer_form_permissions WHERE viewer_id=$1 AND form_id=$2`,
		viewerID, formID,
	).Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetViewerPermissionByID mengambil permission berdasarkan ID-nya.
func (s *Store) GetViewerPermissionByID(ctx context.Context, permID string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at
		 FROM viewer_form_permissions WHERE id=$1`, permID,
	).Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetViewerAllowedRespondentByID mengambil data allowed respondent berdasarkan ID.
func (s *Store) GetViewerAllowedRespondentByID(ctx context.Context, id string) (*models.ViewerAllowedRespondent, error) {
	ar := &models.ViewerAllowedRespondent{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,permission_id,respondent_id,created_at
		 FROM viewer_allowed_respondents WHERE id=$1`, id,
	).Scan(&ar.ID, &ar.PermissionID, &ar.RespondentID, &ar.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ar, err
}

// ListFormViewerPermissions mengembalikan semua permission viewer untuk satu kuesioner (dengan join username).
func (s *Store) ListFormViewerPermissions(ctx context.Context, formID string) ([]models.ViewerFormPermission, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.id, p.viewer_id, p.form_id, p.respondent_access, p.visible_fields,
		        p.created_by, p.created_at, u.username,
		        (SELECT count(*) FROM viewer_allowed_respondents WHERE permission_id=p.id)
		 FROM viewer_form_permissions p
		 JOIN users u ON u.id=p.viewer_id
		 WHERE p.form_id=$1
		 ORDER BY p.created_at`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ViewerFormPermission
	for rows.Next() {
		p := models.ViewerFormPermission{}
		if err := rows.Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields,
			&p.CreatedBy, &p.CreatedAt, &p.ViewerUsername, &p.AllowedCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []models.ViewerFormPermission{}
	}
	return out, rows.Err()
}

// ListViewerForms mengembalikan semua kuesioner yang bisa diakses viewer (dengan join judul form).
func (s *Store) ListViewerForms(ctx context.Context, viewerID string) ([]models.ViewerFormPermission, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.id, p.viewer_id, p.form_id, p.respondent_access, p.visible_fields,
		        p.created_by, p.created_at, f.title
		 FROM viewer_form_permissions p
		 JOIN forms f ON f.id=p.form_id
		 WHERE p.viewer_id=$1
		 ORDER BY f.title`, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ViewerFormPermission
	for rows.Next() {
		p := models.ViewerFormPermission{}
		if err := rows.Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields,
			&p.CreatedBy, &p.CreatedAt, &p.FormTitle); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []models.ViewerFormPermission{}
	}
	return out, rows.Err()
}

// UpdateViewerPermission memperbarui respondent_access dan visible_fields.
func (s *Store) UpdateViewerPermission(ctx context.Context, permID, respondentAccess string, visibleFields []string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	err := s.pool.QueryRow(ctx,
		`UPDATE viewer_form_permissions SET respondent_access=$2, visible_fields=$3
		 WHERE id=$1
		 RETURNING id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at`,
		permID, respondentAccess, visibleFields,
	).Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// DeleteViewerPermission mencabut akses viewer ke kuesioner.
func (s *Store) DeleteViewerPermission(ctx context.Context, permID string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM viewer_form_permissions WHERE id=$1`, permID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddViewerAllowedRespondent menambahkan satu responden ke daftar yang diizinkan.
func (s *Store) AddViewerAllowedRespondent(ctx context.Context, permID, respondentID string) (*models.ViewerAllowedRespondent, error) {
	ar := &models.ViewerAllowedRespondent{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO viewer_allowed_respondents(permission_id,respondent_id)
		 VALUES ($1,$2)
		 ON CONFLICT (permission_id,respondent_id) DO NOTHING
		 RETURNING id,permission_id,respondent_id,created_at`,
		permID, respondentID,
	).Scan(&ar.ID, &ar.PermissionID, &ar.RespondentID, &ar.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ar, err
}

// RemoveViewerAllowedRespondent menghapus satu responden dari daftar yang diizinkan.
func (s *Store) RemoveViewerAllowedRespondent(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM viewer_allowed_respondents WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListViewerAllowedRespondents mengembalikan semua responden yang diizinkan (dengan join email/nama).
func (s *Store) ListViewerAllowedRespondents(ctx context.Context, permID string) ([]models.ViewerAllowedRespondent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ar.id, ar.permission_id, ar.respondent_id, r.email, r.name, ar.created_at
		 FROM viewer_allowed_respondents ar
		 JOIN respondents r ON r.id=ar.respondent_id
		 WHERE ar.permission_id=$1
		 ORDER BY r.name`, permID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ViewerAllowedRespondent
	for rows.Next() {
		ar := models.ViewerAllowedRespondent{}
		if err := rows.Scan(&ar.ID, &ar.PermissionID, &ar.RespondentID, &ar.Email, &ar.Name, &ar.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	if out == nil {
		out = []models.ViewerAllowedRespondent{}
	}
	return out, rows.Err()
}

// ListFormRespondents mengembalikan semua responden yang pernah mengirim jawaban ke kuesioner ini.
func (s *Store) ListFormRespondents(ctx context.Context, formID string) ([]models.Respondent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT r.id, r.google_id, r.email, r.name, r.picture, r.created_at, r.updated_at
		 FROM respondents r
		 JOIN form_responses fr ON fr.respondent_id=r.id
		 WHERE fr.form_id=$1 AND fr.status='submitted'
		 ORDER BY r.name`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Respondent
	for rows.Next() {
		r := models.Respondent{}
		if err := rows.Scan(&r.ID, &r.GoogleID, &r.Email, &r.Name, &r.Picture, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []models.Respondent{}
	}
	return out, rows.Err()
}

// ListViewerResponses mengembalikan jawaban yang boleh dilihat viewer (hanya status='submitted').
// Jika respondent_access='selected', hanya tampilkan responden dalam daftar yang diizinkan.
func (s *Store) ListViewerResponses(ctx context.Context, viewerID, formID string, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	perm, err := s.GetViewerPermission(ctx, viewerID, formID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	sortDir := "DESC"
	if f.SortDir == "asc" {
		sortDir = "ASC"
	}
	sortCol := map[string]string{
		"waktu": "submitted_at",
		"share": "share_id",
		"who":   "meta->>'name'",
	}[f.SortBy]
	if sortCol == "" {
		sortCol = "submitted_at"
	}

	where, wArgs := buildResponseWhere(f)
	args := append([]any{formID}, wArgs...)

	// Gunakan subquery agar tidak perlu manajemen array UUID
	respondentClause := ""
	if perm.RespondentAccess == "selected" {
		n := len(args) + 1 // $1=formID, $2..wArgs, $n=permID
		respondentClause = fmt.Sprintf(
			" AND respondent_id IN (SELECT respondent_id FROM viewer_allowed_respondents WHERE permission_id=$%d)", n)
		args = append(args, perm.ID)
	}

	args = append(args, limit, offset)
	limitN, offsetN := len(args)-1, len(args)

	q := fmt.Sprintf(`
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		FROM form_responses
		WHERE form_id=$1 AND status='submitted'%s%s
		ORDER BY %s %s NULLS LAST
		LIMIT $%d OFFSET $%d`,
		where, respondentClause, sortCol, sortDir, limitN, offsetN)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Response
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return nil, err
		}
		r.Answers = maskAnswers(r.Answers, perm.VisibleFields)
		out = append(out, r)
	}
	if out == nil {
		out = []models.Response{}
	}
	return out, rows.Err()
}

// CountViewerResponses menghitung jawaban yang boleh dilihat viewer.
func (s *Store) CountViewerResponses(ctx context.Context, viewerID, formID string, f ResponseFilter) (int64, error) {
	perm, err := s.GetViewerPermission(ctx, viewerID, formID)
	if err != nil {
		return 0, err
	}

	where, wArgs := buildResponseWhere(f)
	args := append([]any{formID}, wArgs...)

	respondentClause := ""
	if perm.RespondentAccess == "selected" {
		n := len(args) + 1
		respondentClause = fmt.Sprintf(
			" AND respondent_id IN (SELECT respondent_id FROM viewer_allowed_respondents WHERE permission_id=$%d)", n)
		args = append(args, perm.ID)
	}

	var n int64
	err = s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*) FROM form_responses
		WHERE form_id=$1 AND status='submitted'%s%s`,
		where, respondentClause),
		args...,
	).Scan(&n)
	return n, err
}

/* ---------------- editors ---------------- */

// CreateEditorPermission memberikan akses editor ke satu kuesioner.
func (s *Store) CreateEditorPermission(ctx context.Context, editorID, formID string, createdBy *string) (*models.EditorFormPermission, error) {
	p := &models.EditorFormPermission{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO editor_form_permissions(editor_id,form_id,created_by)
		 VALUES ($1,$2,$3)
		 RETURNING id,editor_id,form_id,created_by,created_at`,
		editorID, formID, createdBy,
	).Scan(&p.ID, &p.EditorID, &p.FormID, &p.CreatedBy, &p.CreatedAt)
	return p, err
}

// GetEditorPermissionByID mengambil permission editor berdasarkan ID.
func (s *Store) GetEditorPermissionByID(ctx context.Context, permID string) (*models.EditorFormPermission, error) {
	p := &models.EditorFormPermission{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,editor_id,form_id,created_by,created_at
		 FROM editor_form_permissions WHERE id=$1`, permID,
	).Scan(&p.ID, &p.EditorID, &p.FormID, &p.CreatedBy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ListFormEditorPermissions mengembalikan semua editor yang punya akses ke satu kuesioner.
func (s *Store) ListFormEditorPermissions(ctx context.Context, formID string) ([]models.EditorFormPermission, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.id,p.editor_id,p.form_id,p.created_by,p.created_at,u.username
		 FROM editor_form_permissions p
		 JOIN users u ON u.id=p.editor_id
		 WHERE p.form_id=$1
		 ORDER BY p.created_at`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.EditorFormPermission
	for rows.Next() {
		p := models.EditorFormPermission{}
		if err := rows.Scan(&p.ID, &p.EditorID, &p.FormID, &p.CreatedBy, &p.CreatedAt, &p.EditorName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []models.EditorFormPermission{}
	}
	return out, rows.Err()
}

// DeleteEditorPermission mencabut akses editor dari kuesioner.
func (s *Store) DeleteEditorPermission(ctx context.Context, permID string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM editor_form_permissions WHERE id=$1`, permID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HasEditorFormPermission mengecek apakah editor punya akses kelola ke form tertentu.
func (s *Store) HasEditorFormPermission(ctx context.Context, editorID, formID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM editor_form_permissions WHERE editor_id=$1 AND form_id=$2)`,
		editorID, formID,
	).Scan(&exists)
	return exists, err
}

// maskAnswers menyaring kunci answers JSONB agar hanya field yang diizinkan terlihat.
func maskAnswers(raw json.RawMessage, visible []string) json.RawMessage {
	if len(visible) == 0 {
		return raw
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	allowed := make(map[string]bool, len(visible))
	for _, f := range visible {
		allowed[f] = true
	}
	out := make(map[string]json.RawMessage, len(visible))
	for k, v := range m {
		if allowed[k] {
			out[k] = v
		}
	}
	b, _ := json.Marshal(out)
	return b
}

/* ---------------- wilayah ---------------- */

// GetWilayahByParent mengembalikan daftar wilayah anak dari kodeParent.
// Jika kodeParent kosong, mengembalikan semua wilayah tingkat provinsi (kode_parent IS NULL).
func (s *Store) GetWilayahByParent(ctx context.Context, kodeParent string) ([]models.WilayahItem, error) {
	var rows pgx.Rows
	var err error
	if kodeParent == "" {
		rows, err = s.pool.Query(ctx,
			`SELECT kode_wilayah, nama_wilayah FROM wilayah
			 WHERE kode_parent IS NULL ORDER BY kode_wilayah`)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT kode_wilayah, nama_wilayah FROM wilayah
			 WHERE kode_parent = $1 ORDER BY kode_wilayah`,
			kodeParent)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.WilayahItem
	for rows.Next() {
		var it models.WilayahItem
		if err := rows.Scan(&it.KodeWilayah, &it.NamaWilayah); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if items == nil {
		items = []models.WilayahItem{} // kembalikan [] bukan null
	}
	return items, rows.Err()
}
