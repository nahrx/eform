package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bpskaltim/eform-backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("data tidak ditemukan")

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

/* ---------------- users ---------------- */

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, email, hash, role string) (*models.User, error) {
	var emailArg any
	if email != "" {
		emailArg = email
	}
	u := &models.User{}
	var em *string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users(username,email,password_hash,role) VALUES ($1,$2,$3,$4)
		 RETURNING id,username,email,role,is_active,created_at,updated_at`,
		username, emailArg, hash, role,
	).Scan(&u.ID, &u.Username, &em, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if em != nil {
		u.Email = *em
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

func (s *Store) CreateShare(ctx context.Context, formID, token, label string, allowResponses bool, passwordHash *string, expiresAt *time.Time, createdBy *string) (*models.Share, error) {
	sh := &models.Share{}
	var ph *string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO form_shares(form_id,token,label,allow_responses,password_hash,expires_at,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id,form_id,token,label,is_active,allow_responses,password_hash,expires_at,view_count,created_at`,
		formID, token, label, allowResponses, passwordHash, expiresAt, createdBy,
	).Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &ph, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt)
	if err != nil {
		return nil, err
	}
	sh.HasPassword = ph != nil
	return sh, nil
}

func (s *Store) ListSharesByForm(ctx context.Context, formID string) ([]models.Share, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,form_id,token,label,is_active,allow_responses,password_hash,expires_at,view_count,created_at
		 FROM form_shares WHERE form_id=$1 ORDER BY created_at DESC`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Share
	for rows.Next() {
		sh := models.Share{}
		var ph *string
		if err := rows.Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &ph, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt); err != nil {
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
		`SELECT id,form_id,token,label,is_active,allow_responses,password_hash,expires_at,view_count,created_at
		 FROM form_shares WHERE token=$1`, token,
	).Scan(&sh.ID, &sh.FormID, &sh.Token, &sh.Label, &sh.IsActive, &sh.AllowResponses, &sh.PasswordHash, &sh.ExpiresAt, &sh.ViewCount, &sh.CreatedAt)
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

func (s *Store) IncrementShareView(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE form_shares SET view_count = view_count + 1 WHERE id=$1`, id)
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
		`INSERT INTO form_responses(form_id,share_id,answers,meta) VALUES ($1,$2,$3,$4)
		 RETURNING id,form_id,share_id,respondent_id,answers,meta,submitted_at`,
		formID, shareID, answers, meta,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Answers, &r.Meta, &r.SubmittedAt)
	return r, err
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
		`INSERT INTO form_responses(form_id,share_id,respondent_id,answers,meta)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (form_id,respondent_id) WHERE respondent_id IS NOT NULL
		 DO UPDATE SET share_id=EXCLUDED.share_id, answers=EXCLUDED.answers,
		               meta=EXCLUDED.meta, submitted_at=now()
		 RETURNING id,form_id,share_id,respondent_id,answers,meta,submitted_at`,
		formID, shareID, respondentID, answers, meta,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Answers, &r.Meta, &r.SubmittedAt)
	return r, err
}

// GetResponseByFormAndRespondent mengembalikan jawaban yang pernah dikirim responden untuk form ini.
func (s *Store) GetResponseByFormAndRespondent(ctx context.Context, formID, respondentID string) (*models.Response, error) {
	r := &models.Response{}
	err := s.pool.QueryRow(ctx,
		`SELECT id,form_id,share_id,respondent_id,answers,meta,submitted_at
		 FROM form_responses WHERE form_id=$1 AND respondent_id=$2`,
		formID, respondentID,
	).Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Answers, &r.Meta, &r.SubmittedAt)
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
		`SELECT id,form_id,share_id,respondent_id,answers,meta,submitted_at
		 FROM form_responses WHERE form_id=$1 ORDER BY submitted_at DESC LIMIT $2 OFFSET $3`,
		formID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Response
	for rows.Next() {
		r := models.Response{}
		if err := rows.Scan(&r.ID, &r.FormID, &r.ShareID, &r.RespondentID, &r.Answers, &r.Meta, &r.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CountResponsesByForm(ctx context.Context, formID string) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM form_responses WHERE form_id=$1`, formID).Scan(&n)
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
