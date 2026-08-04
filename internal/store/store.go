package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResponseFilter parameter untuk filter dan sort daftar jawaban admin.
type ResponseFilter struct {
	Status            string              // 'submitted'|'draft'|'' (kosong = semua)
	ShareID           string              // uuid string atau '' (kosong = semua)
	Search            string              // pencarian parsial pada meta.name / meta.email
	SortBy            string              // 'waktu'|'status'|'share'|'who'|nama_field_schema
	SortDir           string              // 'asc'|'desc'
	FieldFilters      map[string]string   // fieldName â†’ nilai teks (ILIKE, untuk field bebas)
	FieldExactFilters map[string]string   // fieldName â†’ nilai pasti (=, untuk dropdown/radio/tanggal)
	FieldAnyFilters   map[string][]string // fieldName â†’ daftar nilai (array berisi salah satu, untuk checkbox/multiselect)
	FieldRangeFilters map[string][2]string // fieldName â†’ [min,max] (rentang angka; salah satu boleh kosong)
}

// Tabel daftar responden yang diizinkan, per jenis pemegang akses. Hanya kedua nilai
// inilah yang boleh masuk ResponseScope.AllowedTable — lihat ResponseScope.clauses.
const (
	AllowedTableViewer = "viewer_allowed_respondents"
	AllowedTableAPIKey = "api_key_allowed_respondents"
)

// ResponseScope adalah pembatasan data yang dipakai bersama oleh akses viewer dan
// akses API key: baris mana yang boleh terlihat (responden terpilih & filter nilai
// variabel) dan kolom mana yang boleh terbaca (VisibleFields).
//
// Sengaja satu bentuk untuk keduanya supaya aturan masking dan pembatasan baris tidak
// bisa lepas sinkron antar jalur akses.
type ResponseScope struct {
	FormID           string
	RespondentAccess string // 'all' | 'selected'
	PermissionID     string // dipakai saat RespondentAccess=='selected'
	AllowedTable     string // AllowedTableViewer | AllowedTableAPIKey
	FieldFilters     map[string]string
	VisibleFields    []string // nil/kosong = semua kolom jawaban
	IncludeDrafts    bool     // false = hanya jawaban yang sudah dikirim
}

// clauses menyusun klausa WHERE tambahan dari scope, melanjutkan penomoran argumen
// yang sudah terpakai.
func (sc ResponseScope) clauses(args []any) (string, []any) {
	clause := ""
	// Jawaban yang belum dikirim ada di dua tempat: tabel response_drafts (dikecualikan
	// lewat source()) dan baris form_responses yang status-nya masih 'draft'. Keduanya
	// harus ditutup di sini, bukan lewat filter dari pemanggil — kalau tidak, jalur yang
	// tidak memakai filter (mis. ekspor CSV) akan ikut membocorkan draft.
	if !sc.IncludeDrafts {
		clause += " AND status='submitted'"
	}
	if sc.RespondentAccess == "selected" {
		if sc.AllowedTable != AllowedTableViewer && sc.AllowedTable != AllowedTableAPIKey {
			// Jangan pernah menyusun SQL dari nama tabel yang tak dikenal — tutup total.
			return " AND false", args
		}
		n := len(args) + 1
		clause = fmt.Sprintf(
			" AND respondent_id IN (SELECT respondent_id FROM %s WHERE permission_id=$%d)", sc.AllowedTable, n)
		args = append(args, sc.PermissionID)
	}
	permClause, args := buildPermissionFieldFilter(sc.FieldFilters, args)
	return clause + permClause, args
}

// source menyusun sumber baris: jawaban terkirim, ditambah draft bila scope mengizinkan.
// $1 selalu formID.
func (sc ResponseScope) source() string {
	base := `SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		    FROM form_responses WHERE form_id=$1`
	if !sc.IncludeDrafts {
		return base
	}
	return base + `
		  UNION ALL
		  SELECT rd.id,rd.form_id,rd.share_id,rd.respondent_id,'draft'::text,rd.answers,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.saved_at
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1`
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
	for fieldName, vals := range f.FieldAnyFilters {
		if isSafeIdentifier(fieldName) && len(vals) > 0 {
			n := add(vals)
			// answers->'field' berisi array JSON (checkbox/multiselect) — cocok bila
			// mengandung SALAH SATU dari nilai yang dipilih di filter (semantik OR).
			where += fmt.Sprintf(" AND answers->'%s' ?| $%d::text[]", fieldName, n)
		}
	}
	for fieldName, bounds := range f.FieldRangeFilters {
		if !isSafeIdentifier(fieldName) {
			continue
		}
		minV, maxV := strings.TrimSpace(bounds[0]), strings.TrimSpace(bounds[1])
		// Validasi angka di sisi Go dulu — nilai ini dikirim sebagai parameter dengan
		// cast eksplisit ::numeric, jadi kalau bukan angka, query akan error saat
		// dieksekusi bila tidak disaring lebih dulu di sini.
		_, minErr := strconv.ParseFloat(minV, 64)
		_, maxErr := strconv.ParseFloat(maxV, 64)
		minOk, maxOk := minV != "" && minErr == nil, maxV != "" && maxErr == nil
		if !minOk && !maxOk {
			continue
		}
		// answers->>'field' bisa berisi teks bukan-angka (jawaban kosong/tak valid) —
		// pola regex jadi penjaga supaya cast ::numeric tidak pernah gagal di tengah query.
		numGuard := fmt.Sprintf("answers->>'%s' ~ '^-?[0-9]+(\\.[0-9]+)?$'", fieldName)
		if minOk {
			n := add(minV)
			where += fmt.Sprintf(" AND %s AND (answers->>'%s')::numeric >= $%d::numeric", numGuard, fieldName, n)
		}
		if maxOk {
			n := add(maxV)
			where += fmt.Sprintf(" AND %s AND (answers->>'%s')::numeric <= $%d::numeric", numGuard, fieldName, n)
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
		`SELECT id,username,email,password_hash,role,is_active,preferred_language,created_at,updated_at
		 FROM users WHERE username=$1`, username,
	).Scan(&u.ID, &u.Username, &em, &u.PasswordHash, &u.Role, &u.IsActive, &u.PreferredLanguage, &u.CreatedAt, &u.UpdatedAt)
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

func (s *Store) GetUser(ctx context.Context, id string) (*models.User, error) {
	u := &models.User{}
	var em *string
	err := s.pool.QueryRow(ctx,
		`SELECT id,username,email,role,is_active,preferred_language,created_at,updated_at FROM users WHERE id=$1`, id,
	).Scan(&u.ID, &u.Username, &em, &u.Role, &u.IsActive, &u.PreferredLanguage, &u.CreatedAt, &u.UpdatedAt)
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
		`SELECT id,username,email,role,is_active,created_at,updated_at FROM users
		 WHERE role IN ('admin','superadmin') ORDER BY created_at`)
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

// UpdateAdminUser mengupdate username, email, dan role user — hanya untuk admin/superadmin.
func (s *Store) UpdateAdminUser(ctx context.Context, id, username, email, role string) error {
	var emailArg any
	if email != "" {
		emailArg = email
	}
	ct, err := s.pool.Exec(ctx,
		`UPDATE users SET username=$1, email=$2, role=$3, updated_at=now()
		 WHERE id=$4 AND role IN ('admin','superadmin')`,
		username, emailArg, role, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserPassword mengupdate password hash user.
func (s *Store) UpdateUserPassword(ctx context.Context, id, hash string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2`,
		hash, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
		`SELECT id,slug,title,description,schema,status,version,owner_id,column_config,created_at,updated_at
		 FROM forms WHERE id=$1`, id,
	).Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Schema, &f.Status, &f.Version, &f.OwnerID, &f.ColumnConfig, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// SaveFormColumnConfig menyimpan konfigurasi kolom tabel jawaban (dipilih admin/superadmin).
func (s *Store) SaveFormColumnConfig(ctx context.Context, formID string, config json.RawMessage) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE forms SET column_config=$2 WHERE id=$1`, formID, config)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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

// ReactivateShare mengaktifkan kembali share yang sebelumnya dicabut.
func (s *Store) ReactivateShare(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `UPDATE form_shares SET is_active=true WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateShare memperbarui konfigurasi share yang masih aktif.
// updatePassword=true  â†’ password_hash diubah ke passwordHash (nil berarti hapus password).
// updateExpiry=true    â†’ expires_at diubah ke expiresAt (nil berarti hapus expiry).
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
	// Fallback: cari di response_drafts (draf form single-response). response_drafts tidak
	// punya kolom meta sendiri, jadi email/nama responden diambil dari tabel respondents.
	err2 := s.pool.QueryRow(ctx,
		`SELECT rd.id,rd.form_id,rd.share_id,rd.respondent_id,rd.answers,
		        jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.saved_at
		 FROM response_drafts rd
		 LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		 WHERE rd.id=$1`, id,
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

// GetFormAnswerColumns mengembalikan semua nama kunci JSONB dari kolom answers untuk satu form.
// Digunakan untuk membangun header CSV sebelum streaming baris.
func (s *Store) GetFormAnswerColumns(ctx context.Context, formID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT jsonb_object_keys(answers)
		FROM (
			SELECT answers FROM form_responses WHERE form_id=$1
			UNION ALL
			SELECT answers FROM response_drafts WHERE form_id=$1
		) sub
		WHERE answers IS NOT NULL AND answers != '{}'::jsonb
		ORDER BY 1`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// GetDistinctFieldValues mengembalikan nilai unik yang benar-benar pernah terisi untuk satu
// variabel di satu form (dipakai sebagai saran nilai filter saat tambah/kelola akses massal).
// Nama field dilewatkan sebagai parameter biasa ke operator jsonb ->> sehingga aman dari injeksi.
func (s *Store) GetDistinctFieldValues(ctx context.Context, formID, fieldName string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT val FROM (
			SELECT answers->>$2 AS val FROM form_responses WHERE form_id=$1
			UNION
			SELECT answers->>$2 AS val FROM response_drafts WHERE form_id=$1
		) sub
		WHERE val IS NOT NULL AND val != ''
		ORDER BY 1
		LIMIT 200`, formID, fieldName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// ForEachResponseByForm memanggil fn untuk setiap respons tanpa batasan jumlah (streaming).
func (s *Store) ForEachResponseByForm(ctx context.Context, formID string, fn func(models.Response) error) error {
	q := `SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		    FROM form_responses WHERE form_id=$1
		  UNION ALL
		  SELECT rd.id,rd.form_id,rd.share_id,rd.respondent_id,'draft'::text,rd.answers,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.saved_at
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1
		) combined ORDER BY submitted_at DESC`
	rows, err := s.pool.Query(ctx, q, formID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return err
		}
		if err := fn(r); err != nil {
			return err
		}
	}
	return rows.Err()
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
	// args: $1=formID, lalu wArgs sebagai $2â€¦$N, lalu limit=$N+1, offset=$N+2
	args := append([]any{formID}, wArgs...)
	args = append(args, limit, offset)
	limitN, offsetN := len(args)-1, len(args)

	q := fmt.Sprintf(`
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		    FROM form_responses WHERE form_id=$1
		  UNION ALL
		  SELECT rd.id,rd.form_id,rd.share_id,rd.respondent_id,'draft'::text,rd.answers,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.saved_at
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1
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
		  SELECT 'draft'::text,rd.share_id,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.answers
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1
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
// newStatus='draft'     â†’ update jawaban saja.
// newStatus='submitted' â†’ update jawaban + submitted_at=now().
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
		return nil, errors.New("sudah ada draf lain â€” selesaikan atau batalkan draf tersebut terlebih dahulu")
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
		`WITH updated AS (
			UPDATE form_responses
			   SET share_id=$2,
			       status='submitted',
			       answers=$4,
			       meta=$5,
			       submitted_at=now()
			 WHERE id = (
			 	SELECT id
			 	  FROM form_responses
			 	 WHERE form_id=$1 AND respondent_id=$3 AND status='submitted'
			 	 ORDER BY submitted_at DESC
			 	 LIMIT 1
			 )
			 RETURNING id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		), inserted AS (
			INSERT INTO form_responses(form_id,share_id,respondent_id,status,answers,meta)
			SELECT $1,$2,$3,'submitted',$4,$5
			WHERE NOT EXISTS (SELECT 1 FROM updated)
			RETURNING id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		)
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM updated
		UNION ALL
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM inserted
		LIMIT 1`,
		formID, shareID, respondentID, answers, meta,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	return r, err
}

// GetResponseByFormAndRespondent mengembalikan jawaban responden untuk form ini (single-response).
// Menyertakan status 'draft' agar respons yang baru dibatalkan pengirimannya (unsubmit)
// tetap bisa dimuat kembali untuk diedit.
func (s *Store) GetResponseByFormAndRespondent(ctx context.Context, formID, respondentID string) (*models.Response, error) {
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		 FROM form_responses
		 WHERE form_id=$1 AND respondent_id=$2 AND status IN ('submitted','draft')
		 ORDER BY submitted_at DESC LIMIT 1`,
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

// UpdateUserNote mengupdate kolom catatan (note) pada user viewer/editor.
func (s *Store) UpdateUserNote(ctx context.Context, id, note string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE users SET note=$1, updated_at=now() WHERE id=$2`,
		note, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserLanguage menyimpan preferensi bahasa UI (builder/dashboard) milik akun sendiri.
func (s *Store) UpdateUserLanguage(ctx context.Context, id, lang string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE users SET preferred_language=$1, updated_at=now() WHERE id=$2`,
		lang, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAdminUser menghapus user admin/superadmin — tidak bisa hapus viewer/editor.
func (s *Store) DeleteAdminUser(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id=$1 AND role IN ('admin','superadmin')`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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

// buildPermissionFieldFilter membuat klausa SQL AND untuk membatasi answers berdasarkan field_filters permission.
// Digunakan di ListViewerResponses, CountViewerResponses, editorListResponses, dll.
func buildPermissionFieldFilter(filters map[string]string, args []any) (string, []any) {
	clause := ""
	for fieldName, val := range filters {
		if isSafeIdentifier(fieldName) && val != "" {
			n := len(args) + 1
			clause += fmt.Sprintf(" AND answers->>'%s'=$%d", fieldName, n)
			args = append(args, val)
		}
	}
	return clause, args
}

func scanViewerPerm(p *models.ViewerFormPermission, row interface {
	Scan(...any) error
}, cols ...any) error {
	var ffRaw json.RawMessage
	all := append(cols, &ffRaw)
	if err := row.Scan(all...); err != nil {
		return err
	}
	if len(ffRaw) > 0 {
		_ = json.Unmarshal(ffRaw, &p.FieldFilters)
	}
	return nil
}

// CreateViewerPermission memberikan akses viewer ke satu kuesioner.
func (s *Store) CreateViewerPermission(ctx context.Context, viewerID, formID, respondentAccess string, visibleFields []string, fieldFilters map[string]string, createdBy *string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	ffBytes, _ := json.Marshal(fieldFilters)
	err := scanViewerPerm(p,
		s.pool.QueryRow(ctx,
			`INSERT INTO viewer_form_permissions(viewer_id,form_id,respondent_access,visible_fields,field_filters,created_by)
			 VALUES ($1,$2,$3,$4,$5,$6)
			 RETURNING id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at,field_filters`,
			viewerID, formID, respondentAccess, visibleFields, ffBytes, createdBy,
		),
		&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt,
	)
	return p, err
}

// GetViewerPermission mengambil permission viewer-form, atau ErrNotFound.
func (s *Store) GetViewerPermission(ctx context.Context, viewerID, formID string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	err := scanViewerPerm(p,
		s.pool.QueryRow(ctx,
			`SELECT id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at,field_filters
			 FROM viewer_form_permissions WHERE viewer_id=$1 AND form_id=$2`,
			viewerID, formID,
		),
		&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetViewerPermissionByID mengambil permission berdasarkan ID-nya.
func (s *Store) GetViewerPermissionByID(ctx context.Context, permID string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	err := scanViewerPerm(p,
		s.pool.QueryRow(ctx,
			`SELECT id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at,field_filters
			 FROM viewer_form_permissions WHERE id=$1`, permID,
		),
		&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt,
	)
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
		        (SELECT count(*) FROM viewer_allowed_respondents WHERE permission_id=p.id),
		        p.field_filters
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
		var ffRaw json.RawMessage
		if err := rows.Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields,
			&p.CreatedBy, &p.CreatedAt, &p.ViewerUsername, &p.AllowedCount, &ffRaw); err != nil {
			return nil, err
		}
		if len(ffRaw) > 0 {
			_ = json.Unmarshal(ffRaw, &p.FieldFilters)
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
		        p.created_by, p.created_at, f.title, p.field_filters
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
		var ffRaw json.RawMessage
		if err := rows.Scan(&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields,
			&p.CreatedBy, &p.CreatedAt, &p.FormTitle, &ffRaw); err != nil {
			return nil, err
		}
		if len(ffRaw) > 0 {
			_ = json.Unmarshal(ffRaw, &p.FieldFilters)
		}
		out = append(out, p)
	}
	if out == nil {
		out = []models.ViewerFormPermission{}
	}
	return out, rows.Err()
}

// UpdateViewerPermission memperbarui respondent_access, visible_fields, dan field_filters.
func (s *Store) UpdateViewerPermission(ctx context.Context, permID, respondentAccess string, visibleFields []string, fieldFilters map[string]string) (*models.ViewerFormPermission, error) {
	p := &models.ViewerFormPermission{}
	ffBytes, _ := json.Marshal(fieldFilters)
	err := scanViewerPerm(p,
		s.pool.QueryRow(ctx,
			`UPDATE viewer_form_permissions SET respondent_access=$2, visible_fields=$3, field_filters=$4
			 WHERE id=$1
			 RETURNING id,viewer_id,form_id,respondent_access,visible_fields,created_by,created_at,field_filters`,
			permID, respondentAccess, visibleFields, ffBytes,
		),
		&p.ID, &p.ViewerID, &p.FormID, &p.RespondentAccess, &p.VisibleFields, &p.CreatedBy, &p.CreatedAt,
	)
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

// ListScopedResponses mengembalikan jawaban yang masuk dalam scope, dengan masking
// VisibleFields. Dipakai jalur viewer dan jalur API key.
func (s *Store) ListScopedResponses(ctx context.Context, sc ResponseScope, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	sortDir := "DESC"
	if f.SortDir == "asc" {
		sortDir = "ASC"
	}
	sortCol := map[string]string{
		"waktu":  "submitted_at",
		"share":  "share_id",
		"who":    "meta->>'name'",
		"status": "status",
	}[f.SortBy]
	if sortCol == "" {
		sortCol = "submitted_at"
	}

	where, wArgs := buildResponseWhere(f)
	args := append([]any{sc.FormID}, wArgs...)
	scopeClause, args := sc.clauses(args)
	args = append(args, limit, offset)
	limitN, offsetN := len(args)-1, len(args)

	q := fmt.Sprintf(`
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  %s
		) combined
		WHERE 1=1%s%s
		ORDER BY %s %s NULLS LAST
		LIMIT $%d OFFSET $%d`,
		sc.source(), where, scopeClause, sortCol, sortDir, limitN, offsetN)

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
		r.Answers = maskAnswers(r.Answers, sc.VisibleFields)
		out = append(out, r)
	}
	if out == nil {
		out = []models.Response{}
	}
	return out, rows.Err()
}

// CountScopedResponses menghitung jawaban yang masuk dalam scope.
func (s *Store) CountScopedResponses(ctx context.Context, sc ResponseScope, f ResponseFilter) (int64, error) {
	where, wArgs := buildResponseWhere(f)
	args := append([]any{sc.FormID}, wArgs...)
	scopeClause, args := sc.clauses(args)

	var n int64
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*) FROM (
		  %s
		) combined
		WHERE 1=1%s%s`, sc.source(), where, scopeClause),
		args...,
	).Scan(&n)
	return n, err
}

// ForEachScopedResponse men-stream semua jawaban dalam scope (tanpa limit/offset dan
// tanpa filter query — sama seperti ekspor CSV admin), dengan masking VisibleFields.
func (s *Store) ForEachScopedResponse(ctx context.Context, sc ResponseScope, fn func(models.Response) error) error {
	args := []any{sc.FormID}
	scopeClause, args := sc.clauses(args)

	q := fmt.Sprintf(`
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  %s
		) combined
		WHERE 1=1%s
		ORDER BY submitted_at DESC`, sc.source(), scopeClause)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return err
		}
		r.Answers = maskAnswers(r.Answers, sc.VisibleFields)
		if err := fn(r); err != nil {
			return err
		}
	}
	return rows.Err()
}

// GetScopedResponseByID mengambil satu jawaban dalam scope. Jawaban di luar scope
// dilaporkan ErrNotFound, bukan error otorisasi, supaya tidak membocorkan keberadaannya.
func (s *Store) GetScopedResponseByID(ctx context.Context, sc ResponseScope, responseID string) (*models.Response, error) {
	resp, err := s.GetResponseByFormAndID(ctx, sc.FormID, responseID)
	if err != nil {
		return nil, err
	}
	if !sc.IncludeDrafts && resp.Status == "draft" {
		return nil, ErrNotFound
	}
	if sc.RespondentAccess == "selected" && resp.RespondentID != nil {
		allowed, err := s.isRespondentAllowedIn(ctx, sc.AllowedTable, sc.PermissionID, *resp.RespondentID)
		if err != nil || !allowed {
			return nil, ErrNotFound
		}
	}
	if !matchesFieldFilters(resp.Answers, sc.FieldFilters) {
		return nil, ErrNotFound
	}
	resp.Answers = maskAnswers(resp.Answers, sc.VisibleFields)
	return resp, nil
}

// ViewerScope menyusun ResponseScope dari permission viewer.
func (s *Store) ViewerScope(ctx context.Context, viewerID, formID string) (ResponseScope, error) {
	perm, err := s.GetViewerPermission(ctx, viewerID, formID)
	if err != nil {
		return ResponseScope{}, err
	}
	return ResponseScope{
		FormID:           formID,
		RespondentAccess: perm.RespondentAccess,
		PermissionID:     perm.ID,
		AllowedTable:     AllowedTableViewer,
		FieldFilters:     perm.FieldFilters,
		VisibleFields:    perm.VisibleFields,
		IncludeDrafts:    true,
	}, nil
}

// ListViewerResponses mengembalikan jawaban yang boleh dilihat viewer.
// Jika respondent_access='selected', hanya tampilkan responden dalam daftar yang diizinkan.
func (s *Store) ListViewerResponses(ctx context.Context, viewerID, formID string, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return nil, err
	}
	return s.ListScopedResponses(ctx, sc, f, limit, offset)
}

// CountViewerResponses menghitung jawaban yang boleh dilihat viewer.
func (s *Store) CountViewerResponses(ctx context.Context, viewerID, formID string, f ResponseFilter) (int64, error) {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return 0, err
	}
	return s.CountScopedResponses(ctx, sc, f)
}

// ForEachViewerResponse men-stream semua respons yang boleh dilihat viewer.
// Dipakai untuk ekspor CSV viewer.
func (s *Store) ForEachViewerResponse(ctx context.Context, viewerID, formID string, fn func(models.Response) error) error {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return err
	}
	return s.ForEachScopedResponse(ctx, sc, fn)
}

// ForEachEditorResponse men-stream semua respons untuk form yang ditugaskan ke editor (tanpa
// limit/offset), dibatasi field_filters permission editor. Dipakai untuk ekspor CSV editor.
func (s *Store) ForEachEditorResponse(ctx context.Context, editorID, formID string, fn func(models.Response) error) error {
	perm, err := s.GetEditorPermissionByEditorAndForm(ctx, editorID, formID)
	if err != nil {
		return err
	}
	args := []any{formID}
	respondentClause := ""
	if perm.RespondentAccess == "selected" {
		n := len(args) + 1
		respondentClause = fmt.Sprintf(
			" AND respondent_id IN (SELECT respondent_id FROM editor_allowed_respondents WHERE permission_id=$%d)", n)
		args = append(args, perm.ID)
	}
	permClause, args := buildPermissionFieldFilter(perm.FieldFilters, args)

	q := fmt.Sprintf(`
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		    FROM form_responses WHERE form_id=$1
		  UNION ALL
		  SELECT rd.id,rd.form_id,rd.share_id,rd.respondent_id,'draft'::text,rd.answers,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.saved_at
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1
		) combined
		WHERE 1=1%s%s
		ORDER BY submitted_at DESC`, respondentClause, permClause)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return err
		}
		if err := fn(r); err != nil {
			return err
		}
	}
	return rows.Err()
}

// GetViewerResponseByID mengambil satu respons untuk viewer, dengan masking visibleFields dan cek respondentAccess.
func (s *Store) GetViewerResponseByID(ctx context.Context, viewerID, formID, responseID string) (*models.Response, error) {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return nil, err
	}
	return s.GetScopedResponseByID(ctx, sc, responseID)
}

// GetResponseByFormAndID mengambil satu respons (submitted atau draft) dari form tertentu.
// Dipakai oleh detail viewer (GetViewerResponseByID) dan editor (GetEditorResponseByID).
func (s *Store) GetResponseByFormAndID(ctx context.Context, formID, responseID string) (*models.Response, error) {
	r := &models.Response{}
	err := s.pool.QueryRow(ctx, `
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		    FROM form_responses WHERE form_id=$1 AND id=$2
		  UNION ALL
		  SELECT rd.id,rd.form_id,rd.share_id,rd.respondent_id,'draft'::text,rd.answers,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.saved_at
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1 AND rd.id=$2
		) combined LIMIT 1`,
		formID, responseID,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Status, &r.Answers, &r.Meta, &r.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// IsRespondentAllowedForViewer mengecek apakah responden tertentu ada dalam daftar izin viewer.
func (s *Store) IsRespondentAllowedForViewer(ctx context.Context, permID, respondentID string) (bool, error) {
	return s.isRespondentAllowedIn(ctx, AllowedTableViewer, permID, respondentID)
}

// isRespondentAllowedIn mengecek keanggotaan responden di salah satu tabel daftar izin.
// table hanya boleh salah satu konstanta AllowedTable* — selain itu selalu false.
func (s *Store) isRespondentAllowedIn(ctx context.Context, table, permID, respondentID string) (bool, error) {
	if table != AllowedTableViewer && table != AllowedTableAPIKey {
		return false, nil
	}
	var exists bool
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE permission_id=$1 AND respondent_id=$2)`, table),
		permID, respondentID,
	).Scan(&exists)
	return exists, err
}

// UpdateResponseAnswers memperbarui jawaban respons (submitted atau draft) oleh editor.
// Tabel target ditentukan dari DB (bukan dari klien) untuk mencegah manipulasi status.
func (s *Store) UpdateResponseAnswers(ctx context.Context, formID, responseID string, answers json.RawMessage) error {
	res, err := s.pool.Exec(ctx,
		`UPDATE form_responses SET answers=$1 WHERE id=$2 AND form_id=$3`,
		answers, responseID, formID)
	if err != nil {
		return err
	}
	if res.RowsAffected() > 0 {
		return nil
	}
	res2, err2 := s.pool.Exec(ctx,
		`UPDATE response_drafts SET answers=$1, saved_at=now() WHERE id=$2 AND form_id=$3`,
		answers, responseID, formID)
	if err2 != nil {
		return err2
	}
	if res2.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteResponseByID menghapus satu jawaban (submitted maupun draft) berdasarkan ID dan formID.
func (s *Store) DeleteResponseByID(ctx context.Context, formID, responseID string) error {
	res, err := s.pool.Exec(ctx,
		`DELETE FROM form_responses WHERE id=$1 AND form_id=$2`,
		responseID, formID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		// Coba di response_drafts
		res2, err2 := s.pool.Exec(ctx,
			`DELETE FROM response_drafts WHERE id=$1 AND form_id=$2`,
			responseID, formID)
		if err2 != nil {
			return err2
		}
		if res2.RowsAffected() == 0 {
			return ErrNotFound
		}
	}
	return nil
}

/* ---------------- editors ---------------- */

// CreateEditorPermission memberikan akses editor ke satu kuesioner.
func (s *Store) CreateEditorPermission(ctx context.Context, editorID, formID, respondentAccess string, fieldFilters map[string]string, createdBy *string) (*models.EditorFormPermission, error) {
	p := &models.EditorFormPermission{}
	ffBytes, _ := json.Marshal(fieldFilters)
	var ffRaw json.RawMessage
	err := s.pool.QueryRow(ctx,
		`INSERT INTO editor_form_permissions(editor_id,form_id,respondent_access,field_filters,created_by)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id,editor_id,form_id,respondent_access,created_by,created_at,field_filters`,
		editorID, formID, respondentAccess, ffBytes, createdBy,
	).Scan(&p.ID, &p.EditorID, &p.FormID, &p.RespondentAccess, &p.CreatedBy, &p.CreatedAt, &ffRaw)
	if err == nil && len(ffRaw) > 0 {
		_ = json.Unmarshal(ffRaw, &p.FieldFilters)
	}
	return p, err
}

// GetEditorPermissionByID mengambil permission editor berdasarkan ID.
func (s *Store) GetEditorPermissionByID(ctx context.Context, permID string) (*models.EditorFormPermission, error) {
	p := &models.EditorFormPermission{}
	var ffRaw json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT id,editor_id,form_id,respondent_access,created_by,created_at,field_filters
		 FROM editor_form_permissions WHERE id=$1`, permID,
	).Scan(&p.ID, &p.EditorID, &p.FormID, &p.RespondentAccess, &p.CreatedBy, &p.CreatedAt, &ffRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil && len(ffRaw) > 0 {
		_ = json.Unmarshal(ffRaw, &p.FieldFilters)
	}
	return p, err
}


// ListFormEditorPermissions mengembalikan semua editor yang punya akses ke satu kuesioner.
func (s *Store) ListFormEditorPermissions(ctx context.Context, formID string) ([]models.EditorFormPermission, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.id, p.editor_id, p.form_id, p.respondent_access, p.created_by, p.created_at, u.username,
		        (SELECT count(*) FROM editor_allowed_respondents WHERE permission_id=p.id),
		        p.field_filters
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
		var ffRaw json.RawMessage
		if err := rows.Scan(&p.ID, &p.EditorID, &p.FormID, &p.RespondentAccess, &p.CreatedBy, &p.CreatedAt,
			&p.EditorName, &p.AllowedCount, &ffRaw); err != nil {
			return nil, err
		}
		if len(ffRaw) > 0 {
			_ = json.Unmarshal(ffRaw, &p.FieldFilters)
		}
		out = append(out, p)
	}
	if out == nil {
		out = []models.EditorFormPermission{}
	}
	return out, rows.Err()
}

// GetEditorPermissionByEditorAndForm mengambil permission editor berdasarkan editorID dan formID.
func (s *Store) GetEditorPermissionByEditorAndForm(ctx context.Context, editorID, formID string) (*models.EditorFormPermission, error) {
	p := &models.EditorFormPermission{}
	var ffRaw json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT id,editor_id,form_id,respondent_access,created_by,created_at,field_filters
		 FROM editor_form_permissions WHERE editor_id=$1 AND form_id=$2`,
		editorID, formID,
	).Scan(&p.ID, &p.EditorID, &p.FormID, &p.RespondentAccess, &p.CreatedBy, &p.CreatedAt, &ffRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil && len(ffRaw) > 0 {
		_ = json.Unmarshal(ffRaw, &p.FieldFilters)
	}
	return p, err
}

// UpdateEditorPermission memperbarui respondent_access dan field_filters permission editor.
func (s *Store) UpdateEditorPermission(ctx context.Context, permID, respondentAccess string, fieldFilters map[string]string) (*models.EditorFormPermission, error) {
	p := &models.EditorFormPermission{}
	ffBytes, _ := json.Marshal(fieldFilters)
	var ffRaw json.RawMessage
	err := s.pool.QueryRow(ctx,
		`UPDATE editor_form_permissions SET respondent_access=$2, field_filters=$3
		 WHERE id=$1
		 RETURNING id,editor_id,form_id,respondent_access,created_by,created_at,field_filters`,
		permID, respondentAccess, ffBytes,
	).Scan(&p.ID, &p.EditorID, &p.FormID, &p.RespondentAccess, &p.CreatedBy, &p.CreatedAt, &ffRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil && len(ffRaw) > 0 {
		_ = json.Unmarshal(ffRaw, &p.FieldFilters)
	}
	return p, err
}

// GetEditorResponseByID mengambil satu respons untuk editor, dengan cek respondentAccess dan field_filters permission.
func (s *Store) GetEditorResponseByID(ctx context.Context, editorID, formID, responseID string) (*models.Response, error) {
	perm, err := s.GetEditorPermissionByEditorAndForm(ctx, editorID, formID)
	if err != nil {
		return nil, err
	}
	resp, err := s.GetResponseByFormAndID(ctx, formID, responseID)
	if err != nil {
		return nil, err
	}
	if perm.RespondentAccess == "selected" && resp.RespondentID != nil {
		allowed, err := s.IsRespondentAllowedForEditor(ctx, perm.ID, *resp.RespondentID)
		if err != nil || !allowed {
			return nil, ErrNotFound
		}
	}
	if !matchesFieldFilters(resp.Answers, perm.FieldFilters) {
		return nil, ErrNotFound
	}
	return resp, nil
}

// GetEditorAllowedRespondentByID mengambil data allowed respondent (editor) berdasarkan ID.
func (s *Store) GetEditorAllowedRespondentByID(ctx context.Context, id string) (*models.EditorAllowedRespondent, error) {
	ar := &models.EditorAllowedRespondent{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,permission_id,respondent_id,created_at
		 FROM editor_allowed_respondents WHERE id=$1`, id,
	).Scan(&ar.ID, &ar.PermissionID, &ar.RespondentID, &ar.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ar, err
}

// AddEditorAllowedRespondent menambahkan satu responden ke daftar yang diizinkan untuk editor.
func (s *Store) AddEditorAllowedRespondent(ctx context.Context, permID, respondentID string) (*models.EditorAllowedRespondent, error) {
	ar := &models.EditorAllowedRespondent{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO editor_allowed_respondents(permission_id,respondent_id)
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

// RemoveEditorAllowedRespondent menghapus satu responden dari daftar yang diizinkan untuk editor.
func (s *Store) RemoveEditorAllowedRespondent(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM editor_allowed_respondents WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListEditorAllowedRespondents mengembalikan semua responden yang diizinkan (dengan join email/nama).
func (s *Store) ListEditorAllowedRespondents(ctx context.Context, permID string) ([]models.EditorAllowedRespondent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ar.id, ar.permission_id, ar.respondent_id, r.email, r.name, ar.created_at
		 FROM editor_allowed_respondents ar
		 JOIN respondents r ON r.id=ar.respondent_id
		 WHERE ar.permission_id=$1
		 ORDER BY r.name`, permID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.EditorAllowedRespondent
	for rows.Next() {
		ar := models.EditorAllowedRespondent{}
		if err := rows.Scan(&ar.ID, &ar.PermissionID, &ar.RespondentID, &ar.Email, &ar.Name, &ar.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	if out == nil {
		out = []models.EditorAllowedRespondent{}
	}
	return out, rows.Err()
}

// IsRespondentAllowedForEditor mengecek apakah responden tertentu ada dalam daftar izin editor.
func (s *Store) IsRespondentAllowedForEditor(ctx context.Context, permID, respondentID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM editor_allowed_respondents WHERE permission_id=$1 AND respondent_id=$2)`,
		permID, respondentID,
	).Scan(&exists)
	return exists, err
}

// ListEditorResponses mengembalikan jawaban yang boleh dikelola editor (dibatasi respondentAccess dan field_filters).
func (s *Store) ListEditorResponses(ctx context.Context, editorID, formID string, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	perm, err := s.GetEditorPermissionByEditorAndForm(ctx, editorID, formID)
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
		"waktu":  "submitted_at",
		"share":  "share_id",
		"who":    "meta->>'name'",
		"status": "status",
	}[f.SortBy]
	if sortCol == "" {
		sortCol = "submitted_at"
	}

	where, wArgs := buildResponseWhere(f)
	args := append([]any{formID}, wArgs...)

	respondentClause := ""
	if perm.RespondentAccess == "selected" {
		n := len(args) + 1
		respondentClause = fmt.Sprintf(
			" AND respondent_id IN (SELECT respondent_id FROM editor_allowed_respondents WHERE permission_id=$%d)", n)
		args = append(args, perm.ID)
	}

	permClause, args := buildPermissionFieldFilter(perm.FieldFilters, args)
	args = append(args, limit, offset)
	limitN, offsetN := len(args)-1, len(args)

	q := fmt.Sprintf(`
		SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at FROM (
		  SELECT id,form_id,share_id,respondent_id,status,answers,meta,submitted_at
		    FROM form_responses WHERE form_id=$1
		  UNION ALL
		  SELECT rd.id,rd.form_id,rd.share_id,rd.respondent_id,'draft'::text,rd.answers,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.saved_at
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1
		) combined
		WHERE 1=1%s%s%s
		ORDER BY %s %s NULLS LAST
		LIMIT $%d OFFSET $%d`,
		where, respondentClause, permClause, sortCol, sortDir, limitN, offsetN)

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
	if out == nil {
		out = []models.Response{}
	}
	return out, rows.Err()
}

// CountEditorResponses menghitung jawaban yang boleh dikelola editor.
func (s *Store) CountEditorResponses(ctx context.Context, editorID, formID string, f ResponseFilter) (int64, error) {
	perm, err := s.GetEditorPermissionByEditorAndForm(ctx, editorID, formID)
	if err != nil {
		return 0, err
	}

	where, wArgs := buildResponseWhere(f)
	args := append([]any{formID}, wArgs...)

	respondentClause := ""
	if perm.RespondentAccess == "selected" {
		n := len(args) + 1
		respondentClause = fmt.Sprintf(
			" AND respondent_id IN (SELECT respondent_id FROM editor_allowed_respondents WHERE permission_id=$%d)", n)
		args = append(args, perm.ID)
	}

	permClause, args := buildPermissionFieldFilter(perm.FieldFilters, args)
	var n int64
	err = s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*) FROM (
		  SELECT status,share_id,meta,answers,respondent_id FROM form_responses WHERE form_id=$1
		  UNION ALL
		  SELECT 'draft'::text,rd.share_id,
		         jsonb_strip_nulls(jsonb_build_object('email',resp.email,'name',resp.name)),rd.answers,rd.respondent_id
		    FROM response_drafts rd
		    LEFT JOIN respondents resp ON resp.id=rd.respondent_id
		    WHERE rd.form_id=$1
		) combined
		WHERE 1=1%s%s%s`, where, respondentClause, permClause),
		args...,
	).Scan(&n)
	return n, err
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

// matchesFieldFilters memeriksa bahwa answers memenuhi semua batasan field_filters (exact match).
func matchesFieldFilters(answers json.RawMessage, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	var m map[string]any
	if err := json.Unmarshal(answers, &m); err != nil {
		return false
	}
	for fieldName, val := range filters {
		if v, ok := m[fieldName]; !ok || fmt.Sprint(v) != val {
			return false
		}
	}
	return true
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

/* ---------------- api keys ---------------- */

// apiKeyCols adalah daftar kolom baku untuk SELECT satu API key, urutannya harus sama
// dengan scanAPIKey.
const apiKeyCols = `id,form_id,label,key_prefix,key_hash,respondent_access,visible_fields,
	field_filters,include_respondent,allowed_ips,rate_limit_per_min,is_active,expires_at,
	last_used_at,last_used_ip,request_count,created_by,created_at`

func scanAPIKey(k *models.FormAPIKey, row interface{ Scan(...any) error }) error {
	var ffRaw json.RawMessage
	var lastIP *string
	if err := row.Scan(&k.ID, &k.FormID, &k.Label, &k.KeyPrefix, &k.KeyHash, &k.RespondentAccess,
		&k.VisibleFields, &ffRaw, &k.IncludeRespondent, &k.AllowedIPs, &k.RateLimitPerMin,
		&k.IsActive, &k.ExpiresAt, &k.LastUsedAt, &lastIP, &k.RequestCount,
		&k.CreatedBy, &k.CreatedAt); err != nil {
		return err
	}
	if lastIP != nil {
		k.LastUsedIP = *lastIP
	}
	if len(ffRaw) > 0 {
		_ = json.Unmarshal(ffRaw, &k.FieldFilters)
	}
	return nil
}

// CreateAPIKey menyimpan API key baru. keyHash adalah SHA-256 hex dari key aslinya —
// key aslinya sendiri tidak pernah masuk ke DB.
func (s *Store) CreateAPIKey(ctx context.Context, k *models.FormAPIKey, keyPrefix, keyHash string, createdBy *string) (*models.FormAPIKey, error) {
	ffBytes, _ := json.Marshal(k.FieldFilters)
	out := &models.FormAPIKey{}
	err := scanAPIKey(out, s.pool.QueryRow(ctx,
		`INSERT INTO form_api_keys(form_id,label,key_prefix,key_hash,respondent_access,visible_fields,
			field_filters,include_respondent,allowed_ips,rate_limit_per_min,expires_at,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 RETURNING `+apiKeyCols,
		k.FormID, k.Label, keyPrefix, keyHash, k.RespondentAccess, k.VisibleFields,
		ffBytes, k.IncludeRespondent, k.AllowedIPs, k.RateLimitPerMin, k.ExpiresAt, createdBy,
	))
	return out, err
}

// ListAPIKeysByForm mengembalikan semua API key satu kuesioner beserta jumlah responden terpilih.
func (s *Store) ListAPIKeysByForm(ctx context.Context, formID string) ([]models.FormAPIKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+apiKeyCols+`,
		        (SELECT count(*) FROM api_key_allowed_respondents WHERE permission_id=form_api_keys.id)
		 FROM form_api_keys WHERE form_id=$1 ORDER BY created_at DESC`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.FormAPIKey
	for rows.Next() {
		k := models.FormAPIKey{}
		var ffRaw json.RawMessage
		var lastIP *string
		if err := rows.Scan(&k.ID, &k.FormID, &k.Label, &k.KeyPrefix, &k.KeyHash, &k.RespondentAccess,
			&k.VisibleFields, &ffRaw, &k.IncludeRespondent, &k.AllowedIPs, &k.RateLimitPerMin,
			&k.IsActive, &k.ExpiresAt, &k.LastUsedAt, &lastIP, &k.RequestCount,
			&k.CreatedBy, &k.CreatedAt, &k.AllowedCount); err != nil {
			return nil, err
		}
		if lastIP != nil {
			k.LastUsedIP = *lastIP
		}
		if len(ffRaw) > 0 {
			_ = json.Unmarshal(ffRaw, &k.FieldFilters)
		}
		out = append(out, k)
	}
	if out == nil {
		out = []models.FormAPIKey{}
	}
	return out, rows.Err()
}

// GetAPIKeyByID mengambil satu API key berdasarkan ID-nya.
func (s *Store) GetAPIKeyByID(ctx context.Context, id string) (*models.FormAPIKey, error) {
	k := &models.FormAPIKey{}
	err := scanAPIKey(k, s.pool.QueryRow(ctx, `SELECT `+apiKeyCols+` FROM form_api_keys WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return k, err
}

// GetAPIKeyByHash adalah jalur lookup saat autentikasi: key yang dikirim klien di-hash
// lalu dicari lewat index unik key_hash.
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.FormAPIKey, error) {
	k := &models.FormAPIKey{}
	err := scanAPIKey(k, s.pool.QueryRow(ctx, `SELECT `+apiKeyCols+` FROM form_api_keys WHERE key_hash=$1`, keyHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return k, err
}

// UpdateAPIKey memperbarui label dan seluruh pengaturan cakupan/keamanan satu key.
// key_hash tidak pernah ikut berubah di sini — itu urusan RotateAPIKey.
func (s *Store) UpdateAPIKey(ctx context.Context, id string, k *models.FormAPIKey) (*models.FormAPIKey, error) {
	ffBytes, _ := json.Marshal(k.FieldFilters)
	out := &models.FormAPIKey{}
	err := scanAPIKey(out, s.pool.QueryRow(ctx,
		`UPDATE form_api_keys SET label=$2, respondent_access=$3, visible_fields=$4, field_filters=$5,
		        include_respondent=$6, allowed_ips=$7, rate_limit_per_min=$8, is_active=$9, expires_at=$10
		 WHERE id=$1 RETURNING `+apiKeyCols,
		id, k.Label, k.RespondentAccess, k.VisibleFields, ffBytes,
		k.IncludeRespondent, k.AllowedIPs, k.RateLimitPerMin, k.IsActive, k.ExpiresAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

// RotateAPIKey mengganti kredensial satu key tanpa mengubah cakupannya. Key lama
// langsung tidak berlaku karena key_hash-nya ditimpa.
func (s *Store) RotateAPIKey(ctx context.Context, id, keyPrefix, keyHash string) (*models.FormAPIKey, error) {
	out := &models.FormAPIKey{}
	err := scanAPIKey(out, s.pool.QueryRow(ctx,
		`UPDATE form_api_keys SET key_prefix=$2, key_hash=$3, request_count=0,
		        last_used_at=NULL, last_used_ip=NULL
		 WHERE id=$1 RETURNING `+apiKeyCols,
		id, keyPrefix, keyHash,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

// DeleteAPIKey menghapus API key beserta daftar respondennya (ON DELETE CASCADE).
func (s *Store) DeleteAPIKey(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM form_api_keys WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchAPIKey mencatat pemakaian terakhir sebuah key. Kegagalan di sini sengaja tidak
// menggagalkan permintaan — pemanggilnya cukup mengabaikan error.
func (s *Store) TouchAPIKey(ctx context.Context, id, ip string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE form_api_keys SET last_used_at=now(), last_used_ip=$2, request_count=request_count+1
		 WHERE id=$1`, id, ip)
	return err
}

// APIKeyScope menyusun ResponseScope dari sebuah API key. Draft tidak pernah ikut:
// API hanya membagikan jawaban yang sudah dikirim.
func APIKeyScope(k *models.FormAPIKey) ResponseScope {
	return ResponseScope{
		FormID:           k.FormID,
		RespondentAccess: k.RespondentAccess,
		PermissionID:     k.ID,
		AllowedTable:     AllowedTableAPIKey,
		FieldFilters:     k.FieldFilters,
		VisibleFields:    k.VisibleFields,
		IncludeDrafts:    false,
	}
}

// ListAPIKeyAllowedRespondents mengembalikan responden yang diizinkan (dengan join email/nama).
func (s *Store) ListAPIKeyAllowedRespondents(ctx context.Context, permID string) ([]models.APIKeyAllowedRespondent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ar.id, ar.permission_id, ar.respondent_id, r.email, r.name, ar.created_at
		 FROM api_key_allowed_respondents ar
		 JOIN respondents r ON r.id=ar.respondent_id
		 WHERE ar.permission_id=$1
		 ORDER BY r.name`, permID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.APIKeyAllowedRespondent
	for rows.Next() {
		ar := models.APIKeyAllowedRespondent{}
		if err := rows.Scan(&ar.ID, &ar.PermissionID, &ar.RespondentID, &ar.Email, &ar.Name, &ar.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	if out == nil {
		out = []models.APIKeyAllowedRespondent{}
	}
	return out, rows.Err()
}

// AddAPIKeyAllowedRespondent menambahkan satu responden ke daftar yang diizinkan.
func (s *Store) AddAPIKeyAllowedRespondent(ctx context.Context, permID, respondentID string) (*models.APIKeyAllowedRespondent, error) {
	ar := &models.APIKeyAllowedRespondent{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO api_key_allowed_respondents(permission_id,respondent_id)
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

// GetAPIKeyAllowedRespondentByID dipakai untuk mengecek kepemilikan sebelum menghapus.
func (s *Store) GetAPIKeyAllowedRespondentByID(ctx context.Context, id string) (*models.APIKeyAllowedRespondent, error) {
	ar := &models.APIKeyAllowedRespondent{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,permission_id,respondent_id,created_at FROM api_key_allowed_respondents WHERE id=$1`, id,
	).Scan(&ar.ID, &ar.PermissionID, &ar.RespondentID, &ar.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ar, err
}

// RemoveAPIKeyAllowedRespondent menghapus satu responden dari daftar yang diizinkan.
func (s *Store) RemoveAPIKeyAllowedRespondent(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM api_key_allowed_respondents WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// InsertAPIAccessLog mencatat satu panggilan /api/v1. Dipanggil juga untuk permintaan
// yang ditolak — apiKeyID/formID boleh nil kalau key-nya tidak dikenal.
func (s *Store) InsertAPIAccessLog(ctx context.Context, l *models.APIAccessLog) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO api_access_logs(api_key_id,key_prefix,form_id,ip,path,query,status,row_count,error)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.APIKeyID, l.KeyPrefix, l.FormID, l.IP, l.Path, l.Query, l.Status, l.RowCount, l.Error)
	return err
}

// ListAPIAccessLogs mengembalikan riwayat panggilan satu key, terbaru dulu.
func (s *Store) ListAPIAccessLogs(ctx context.Context, apiKeyID string, limit int) ([]models.APIAccessLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id,api_key_id,key_prefix,form_id,ip,path,query,status,row_count,error,created_at
		 FROM api_access_logs WHERE api_key_id=$1 ORDER BY created_at DESC LIMIT $2`, apiKeyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.APIAccessLog
	for rows.Next() {
		l := models.APIAccessLog{}
		if err := rows.Scan(&l.ID, &l.APIKeyID, &l.KeyPrefix, &l.FormID, &l.IP, &l.Path,
			&l.Query, &l.Status, &l.RowCount, &l.Error, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if out == nil {
		out = []models.APIAccessLog{}
	}
	return out, rows.Err()
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
