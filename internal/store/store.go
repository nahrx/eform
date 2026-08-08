package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nahrx/eform/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResponseFilter carries the filter and sort parameters for the admin response list.
type ResponseFilter struct {
	Status            string              // 'submitted'|'draft'|'' (empty = all)
	ShareID           string              // uuid string or '' (empty = all)
	Search            string              // partial search over meta.name / meta.email
	SortBy            string              // 'time'|'status'|'share'|'who'|schema field name
	SortDir           string              // 'asc'|'desc'
	FieldFilters      map[string]string   // fieldName → text value (ILIKE, for free-text fields)
	FieldExactFilters map[string]string   // fieldName → exact value (=, for dropdown/radio/date)
	FieldAnyFilters   map[string][]string // fieldName → list of values (array contains any of them, for checkbox/multiselect)
	FieldRangeFilters map[string][2]string // fieldName → [min,max] (numeric range; either bound may be empty)
}

// Allowed-respondent tables, one per kind of access holder. These are the only two
// values that may reach ResponseScope.AllowedTable — see ResponseScope.clauses.
const (
	AllowedTableViewer = "viewer_allowed_respondents"
	AllowedTableAPIKey = "api_key_allowed_respondents"
)

// ResponseScope is the data restriction shared by viewer access and API-key access:
// which rows may be seen (selected respondents & field-value filters) and which
// columns may be read (VisibleFields).
//
// Deliberately a single shape for both, so the masking and row-restriction rules
// cannot drift apart between the two access paths.
type ResponseScope struct {
	FormID           string
	RespondentAccess string // 'all' | 'selected'
	PermissionID     string // used when RespondentAccess=='selected'
	AllowedTable     string // AllowedTableViewer | AllowedTableAPIKey
	FieldFilters     map[string]string
	VisibleFields    []string // nil/empty = every answer column
	IncludeDrafts    bool     // false = submitted responses only
}

// clauses builds the extra WHERE clauses from the scope, continuing the argument
// numbering already in use.
func (sc ResponseScope) clauses(args []any) (string, []any) {
	clause := ""
	// Unsubmitted answers live in two places: the response_drafts table (excluded via
	// source()) and form_responses rows still marked 'draft'. Both must be closed off
	// here rather than through a caller-supplied filter — otherwise a path that applies
	// no filter (CSV export, for instance) would leak drafts.
	if !sc.IncludeDrafts {
		clause += " AND status='submitted'"
	}
	if sc.RespondentAccess == "selected" {
		if sc.AllowedTable != AllowedTableViewer && sc.AllowedTable != AllowedTableAPIKey {
			// Never build SQL from an unrecognised table name — deny outright.
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

// source builds the row source: submitted responses, plus drafts when the scope allows it.
// $1 is always formID.
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

// isSafeIdentifier validates a schema field name so it is safe to interpolate into SQL.
// Only letters, digits, and underscores are allowed.
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

// buildResponseWhere builds the WHERE clause and args slice for the response list/count
// queries. Arguments start at $2, because $1 is always formID.
func buildResponseWhere(f ResponseFilter) (string, []any) {
	var args []any
	where := ""
	add := func(v any) int {
		args = append(args, v)
		return len(args) + 1 // +1 because $1=formID sits outside this slice
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
			// answers->'field' holds a JSON array (checkbox/multiselect) — it matches when it
			// contains ANY of the values selected in the filter (OR semantics).
			where += fmt.Sprintf(" AND answers->'%s' ?| $%d::text[]", fieldName, n)
		}
	}
	for fieldName, bounds := range f.FieldRangeFilters {
		if !isSafeIdentifier(fieldName) {
			continue
		}
		minV, maxV := strings.TrimSpace(bounds[0]), strings.TrimSpace(bounds[1])
		// Validate the number on the Go side first — the value is passed as a parameter
		// with an explicit ::numeric cast, so a non-numeric value would fail at execution
		// time unless it is filtered out here.
		_, minErr := strconv.ParseFloat(minV, 64)
		_, maxErr := strconv.ParseFloat(maxV, 64)
		minOk, maxOk := minV != "" && minErr == nil, maxV != "" && maxErr == nil
		if !minOk && !maxOk {
			continue
		}
		// answers->>'field' may hold non-numeric text (empty or invalid answers) — the regex
		// guard keeps the ::numeric cast from ever failing mid-query.
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

var ErrNotFound = errors.New("data not found")

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
		`SELECT id,username,email,password_hash,role,is_active,preferred_language,token_version,created_at,updated_at
		 FROM users WHERE username=$1`, username,
	).Scan(&u.ID, &u.Username, &em, &u.PasswordHash, &u.Role, &u.IsActive, &u.PreferredLanguage, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt)
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
		`SELECT id,username,email,role,is_active,preferred_language,token_version,created_at,updated_at FROM users WHERE id=$1`, id,
	).Scan(&u.ID, &u.Username, &em, &u.Role, &u.IsActive, &u.PreferredLanguage, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt)
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

// AuthSnapshot is the minimum data authMW needs to decide whether a token may still
// be used.
type AuthSnapshot struct {
	IsActive     bool
	Role         string
	TokenVersion int
}

// GetAuthSnapshot runs on every JWT-bearing request. Deliberately a single-row lookup
// by primary key, so account deactivation, deletion, password changes, and
// a role change takes effect immediately — rather than waiting for the token to expire.
func (s *Store) GetAuthSnapshot(ctx context.Context, id string) (*AuthSnapshot, error) {
	a := &AuthSnapshot{}
	err := s.pool.QueryRow(ctx,
		`SELECT is_active,role,token_version FROM users WHERE id=$1`, id,
	).Scan(&a.IsActive, &a.Role, &a.TokenVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// BumpTokenVersion revokes every session belonging to one user.
func (s *Store) BumpTokenVersion(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET token_version=token_version+1 WHERE id=$1`, id)
	return err
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

// UpdateAdminUser updates a user's username, email, and role — admin/superadmin only.
// A role change bumps token_version so the privileges on in-flight sessions do not
// linger at the old level.
func (s *Store) UpdateAdminUser(ctx context.Context, id, username, email, role string) error {
	var emailArg any
	if email != "" {
		emailArg = email
	}
	ct, err := s.pool.Exec(ctx,
		`UPDATE users SET username=$1, email=$2, role=$3, updated_at=now(),
		        token_version=token_version+CASE WHEN role<>$3 THEN 1 ELSE 0 END
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

// UpdateUserPassword mengupdate password hash user. token_version ikut dinaikkan
// so sessions still using the old password are cut off immediately.
func (s *Store) UpdateUserPassword(ctx context.Context, id, hash string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1, token_version=token_version+1, updated_at=now() WHERE id=$2`,
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

// SaveFormColumnConfig stores the response-table column configuration chosen by an admin/superadmin.
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

// ListForms does not return the schema (keeps the list payload small).
// listFormsQuery is the canonical form-list shape: the form columns plus a response count
// submitted, counted in the same sub-query.
//
// The count covers submitted responses AND drafts — matching the number on the form
// management page, so the two pages never disagree.
// It rides along in this query so the dashboard need not call a count endpoint once
// per form (previously N+1 HTTP requests from the browser).
const listFormsQuery = `SELECT f.id,f.slug,f.title,f.description,f.status,f.version,f.owner_id,
	       f.created_at,f.updated_at,
	       (SELECT count(*) FROM form_responses fr WHERE fr.form_id=f.id)
	     + (SELECT count(*) FROM response_drafts rd WHERE rd.form_id=f.id)
	FROM forms f`

func scanFormRows(rows pgx.Rows) ([]models.Form, error) {
	defer rows.Close()
	var out []models.Form
	for rows.Next() {
		f := models.Form{}
		if err := rows.Scan(&f.ID, &f.Slug, &f.Title, &f.Description, &f.Status, &f.Version,
			&f.OwnerID, &f.CreatedAt, &f.UpdatedAt, &f.ResponseCount); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) ListForms(ctx context.Context) ([]models.Form, error) {
	rows, err := s.pool.Query(ctx, listFormsQuery+` ORDER BY f.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	return scanFormRows(rows)
}

// ListFormsByOwner returns the forms belonging to a given owner.
func (s *Store) ListFormsByOwner(ctx context.Context, ownerID string) ([]models.Form, error) {
	rows, err := s.pool.Query(ctx, listFormsQuery+` WHERE f.owner_id=$1 ORDER BY f.updated_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	return scanFormRows(rows)
}

// ListFormsByEditor returns the forms assigned to an editor.
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

// ---- instrument schema versions -------------------------------------------------

// EnsureSchemaVersion records the form's schema as a snapshot and returns its id,
// reusing the existing row when that exact schema was already captured.
//
// Identity is the content hash, not the version string: admins routinely save without
// bumping the version, so keying on the label would fold genuinely different
// instruments into one snapshot and defeat the point of keeping them.
func (s *Store) EnsureSchemaVersion(ctx context.Context, formID, version string, schema json.RawMessage) (string, error) {
	if len(schema) == 0 {
		schema = json.RawMessage(`{}`)
	}
	var id string
	err := s.pool.QueryRow(ctx,
		`WITH v AS (SELECT $3::jsonb AS s)
		 INSERT INTO form_schema_versions(form_id, version, schema, schema_hash)
		 SELECT $1, $2, v.s, md5(v.s::text) FROM v
		 ON CONFLICT (form_id, schema_hash)
		   -- a no-op update, so the row that already exists is still RETURNED
		   DO UPDATE SET form_id = form_schema_versions.form_id
		 RETURNING id`,
		formID, version, schema,
	).Scan(&id)
	return id, err
}

// CurrentSchemaVersionID returns the newest snapshot for a form, used as the fallback
// when a submission does not name the version it was filled against.
func (s *Store) CurrentSchemaVersionID(ctx context.Context, formID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM form_schema_versions
		 WHERE form_id=$1 ORDER BY created_at DESC LIMIT 1`, formID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

// SchemaVersionBelongsToForm guards the version id a client sends back with a
// submission. The id only labels data, but a respondent must still not be able to pin
// their answers to another form's instrument.
func (s *Store) SchemaVersionBelongsToForm(ctx context.Context, formID, versionID string) bool {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT 1 FROM form_schema_versions WHERE id=$1 AND form_id=$2`, versionID, formID,
	).Scan(&n)
	return err == nil
}

// PinResponseSchemaVersion records which instrument a response was filled against.
// It only ever fills an empty pin: a response's history must not be rewritten later.
func (s *Store) PinResponseSchemaVersion(ctx context.Context, responseID, versionID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE form_responses SET schema_version_id=$2
		 WHERE id=$1 AND schema_version_id IS NULL`, responseID, versionID)
	return err
}

// ResponseSchemaInfo describes the instrument a response was filled against, and
// whether the form has been edited since.
type ResponseSchemaInfo struct {
	Known    bool      `json:"known"`
	Version  string    `json:"version"`
	PinnedAt time.Time `json:"pinnedAt"`
	Outdated bool      `json:"outdated"`
	// True when the snapshot was created after the response was submitted, which can
	// only mean the pin was assigned retroactively by the 0020 backfill rather than
	// recorded at the time. Such a pin says what the instrument looks like now, not
	// what was asked then — a distinction worth surfacing rather than papering over.
	Backfilled bool `json:"backfilled"`
}

// GetResponseSchemaInfo answers "was this response filled against the instrument I am
// looking at now?" — what the response detail page needs in order to warn that what it
// renders may no longer match what was actually asked.
func (s *Store) GetResponseSchemaInfo(ctx context.Context, responseID string) (*ResponseSchemaInfo, error) {
	info := &ResponseSchemaInfo{}
	err := s.pool.QueryRow(ctx,
		`SELECT v.version, v.created_at,
		        (v.schema_hash IS DISTINCT FROM md5(f.schema::text)) AS outdated,
		        (v.created_at > r.submitted_at)                      AS backfilled
		 FROM form_responses r
		 JOIN form_schema_versions v ON v.id = r.schema_version_id
		 JOIN forms f ON f.id = r.form_id
		 WHERE r.id = $1`, responseID,
	).Scan(&info.Version, &info.PinnedAt, &info.Outdated, &info.Backfilled)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the response does not exist, or it predates pinning and the backfill
		// never reached it. Both mean "we cannot say", which is not an error.
		return &ResponseSchemaInfo{Known: false}, nil
	}
	if err != nil {
		return nil, err
	}
	info.Known = true
	return info, nil
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

// GetShareByID fetches a single share by its ID.
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

// ReactivateShare re-enables a share that was previously revoked.
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

// UpdateShare updates the configuration of a still-active share.
// updatePassword=true  → password_hash is set to passwordHash (nil removes the password).
// updateExpiry=true    → expires_at is set to expiresAt (nil removes the expiry).
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

// DeleteShare permanently removes a share that has already been revoked (is_active=false).
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

// GetShareAllowedEmailByID fetches one email-allowlist entry by ID.
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

// CreateMultiResponseRow inserts a new form_responses row with a given status ('submitted' or 'draft').
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

// GetResponseByID fetches a single response by its ID.
// If it is not in form_responses, look in response_drafts (the single-response draft).
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
	// Fallback: look in response_drafts (single-response form drafts). response_drafts has
	// no meta column of its own, so the respondent's email/name come from the respondents table.
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

// GetFormAnswerColumns returns every JSONB key present in the answers column for a form.
// Used to build the CSV header before streaming rows.
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

// GetDistinctFieldValues returns the distinct values that have actually been recorded for
// one field of one form (used to suggest filter values in the bulk access dialogs).
// The field name is passed as an ordinary parameter to the jsonb ->> operator, so it is
// safe from injection.
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

// ForEachResponseByForm calls fn for every response with no row limit (streaming).
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

// ListAllResponsesByForm returns every response (form_responses + response_drafts) for the
// admin view. It supports status/shareId/search/per-field filters and dynamic sorting,
// including by schema field.
func (s *Store) ListAllResponsesByForm(ctx context.Context, formID string, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	// Sort column: check the fixed allowlist first, then try it as a schema field name
	sortDir := "DESC"
	if f.SortDir == "asc" {
		sortDir = "ASC"
	}
	sortCol := map[string]string{
		"time":   "submitted_at",
		"waktu":  "submitted_at", // legacy value still sent by cached pages
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
	// args: $1=formID, then wArgs as $2…$N, then limit=$N+1, offset=$N+2
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

// CountAllResponsesByForm counts every response (all statuses + response_drafts) matching the filter.
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

// HasDraftResponse reports whether the respondent still has an active draft for this form.
func (s *Store) HasDraftResponse(ctx context.Context, formID, respondentID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM form_responses WHERE form_id=$1 AND respondent_id=$2 AND status='draft')`,
		formID, respondentID,
	).Scan(&exists)
	return exists, err
}

// UpdateMultiResponseDraft updates an existing draft response.
// newStatus='draft'     → update the answers only.
// newStatus='submitted' → update the answers and set submitted_at=now().
// Succeeds only if the row is still 'draft' and belongs to respondentID on the same formID.
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

// UnsubmitResponse moves a response from 'submitted' back to 'draft' so it can be edited.
// It fails if the same respondent already has another draft for the same form.
func (s *Store) UnsubmitResponse(ctx context.Context, id, respondentID, formID string) (*models.Response, error) {
	var draftExists bool
	_ = s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM form_responses WHERE form_id=$1 AND respondent_id=$2 AND status='draft' AND id!=$3)`,
		formID, respondentID, id,
	).Scan(&draftExists)
	if draftExists {
		return nil, errors.New("another draft already exists — please finish or discard it first")
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

// ListResponsesByFormAndRespondent returns all of a respondent's answers for this form (multi-response).
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

// UpsertResponse stores an answer bound to a respondent.
// If an answer already exists for (form_id, respondent_id), it is updated.
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

// GetResponseByFormAndRespondent returns a respondent's answer for this form (single-response).
// It includes 'draft' status so a response that was just unsubmitted can still be loaded
// again for editing.
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

// UpsertRespondent creates or updates a respondent record keyed by google_id.
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

// GetUserByEmail looks a user up by email address (used for viewer Google OAuth).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	var em *string
	err := s.pool.QueryRow(ctx,
		`SELECT id,username,email,password_hash,role,is_active,token_version,created_at,updated_at
		 FROM users WHERE lower(email)=lower($1)`, email,
	).Scan(&u.ID, &u.Username, &em, &u.PasswordHash, &u.Role, &u.IsActive, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt)
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

// UpdateUserNote updates the note column on a viewer/editor user.
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

// UpdateUserLanguage stores the account's own UI language preference (builder/dashboard).
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

// DeleteAdminUser deletes an admin/superadmin user — it cannot delete viewers/editors.
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

// DeleteUser permanently deletes a user (intended for viewers).
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

// buildPermissionFieldFilter builds the AND clause that restricts answers according to a
// permission's field_filters.
// Used by ListViewerResponses, CountViewerResponses, editorListResponses, and others.
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

// CreateViewerPermission grants a viewer access to one form.
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

// GetViewerPermission fetches a viewer-form permission, or ErrNotFound.
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

// GetViewerPermissionByID fetches a permission by its ID.
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

// GetViewerAllowedRespondentByID fetches an allowed-respondent row by its ID.
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

// ListFormViewerPermissions returns every viewer permission for one form (joined with the username).
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

// ListViewerForms returns every form a viewer can access (joined with the form title).
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

// UpdateViewerPermission updates respondent_access, visible_fields, and field_filters.
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

// DeleteViewerPermission revokes a viewer's access to a form.
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

// AddViewerAllowedRespondent adds one respondent to the allowed list.
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

// RemoveViewerAllowedRespondent removes one respondent from the allowed list.
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

// ListViewerAllowedRespondents returns every allowed respondent (joined with email/name).
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

// ListFormRespondents returns every respondent who has ever submitted an answer to this form.
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

// ListScopedResponses returns the answers that fall inside the scope, with VisibleFields
// masking applied. Used by both the viewer path and the API-key path.
func (s *Store) ListScopedResponses(ctx context.Context, sc ResponseScope, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	sortDir := "DESC"
	if f.SortDir == "asc" {
		sortDir = "ASC"
	}
	sortCol := map[string]string{
		"time":   "submitted_at",
		"waktu":  "submitted_at", // legacy value still sent by cached pages
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

// CountScopedResponses counts the answers that fall inside the scope.
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

// ForEachScopedResponse streams every answer in scope (no limit/offset and no query
// filter — just like the admin CSV export), with VisibleFields masking applied.
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

// GetScopedResponseByID fetches one answer within the scope. An answer outside the scope
// is reported as ErrNotFound rather than an authorisation error, so its existence is not
// disclosed.
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

// ViewerScope builds a ResponseScope from a viewer permission.
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

// ListViewerResponses returns the answers a viewer is allowed to see.
// When respondent_access='selected', only respondents on the allowed list are shown.
func (s *Store) ListViewerResponses(ctx context.Context, viewerID, formID string, f ResponseFilter, limit, offset int) ([]models.Response, error) {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return nil, err
	}
	return s.ListScopedResponses(ctx, sc, f, limit, offset)
}

// CountViewerResponses counts the answers a viewer is allowed to see.
func (s *Store) CountViewerResponses(ctx context.Context, viewerID, formID string, f ResponseFilter) (int64, error) {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return 0, err
	}
	return s.CountScopedResponses(ctx, sc, f)
}

// ForEachViewerResponse streams every response a viewer may see.
// Used for the viewer CSV export.
func (s *Store) ForEachViewerResponse(ctx context.Context, viewerID, formID string, fn func(models.Response) error) error {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return err
	}
	return s.ForEachScopedResponse(ctx, sc, fn)
}

// ForEachEditorResponse streams every response for a form assigned to an editor (no
// limit/offset), restricted by the editor permission's field_filters. Used for the editor
// CSV export.
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

// GetViewerResponseByID fetches one response for a viewer, applying visibleFields masking
// and the respondentAccess check.
func (s *Store) GetViewerResponseByID(ctx context.Context, viewerID, formID, responseID string) (*models.Response, error) {
	sc, err := s.ViewerScope(ctx, viewerID, formID)
	if err != nil {
		return nil, err
	}
	return s.GetScopedResponseByID(ctx, sc, responseID)
}

// GetResponseByFormAndID fetches one response (submitted or draft) from a given form.
// Used by the viewer detail path (GetViewerResponseByID) and the editor one
// (GetEditorResponseByID).
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

// IsRespondentAllowedForViewer reports whether a given respondent is on the viewer's allowed list.
func (s *Store) IsRespondentAllowedForViewer(ctx context.Context, permID, respondentID string) (bool, error) {
	return s.isRespondentAllowedIn(ctx, AllowedTableViewer, permID, respondentID)
}

// isRespondentAllowedIn checks a respondent's membership in one of the allowed-list tables.
// table must be one of the AllowedTable* constants — anything else always returns false.
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

// UpdateResponseAnswers updates a response's answers (submitted or draft) on behalf of an
// editor. The target table is decided from the database, never from the client, to prevent
// status tampering.
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

// DeleteResponseByID deletes one answer (submitted or draft) by ID and formID.
func (s *Store) DeleteResponseByID(ctx context.Context, formID, responseID string) error {
	res, err := s.pool.Exec(ctx,
		`DELETE FROM form_responses WHERE id=$1 AND form_id=$2`,
		responseID, formID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		// Try response_drafts
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

// CreateEditorPermission grants an editor access to one form.
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

// GetEditorPermissionByID fetches an editor permission by its ID.
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


// ListFormEditorPermissions returns every editor with access to one form.
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

// GetEditorPermissionByEditorAndForm fetches an editor permission by editorID and formID.
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

// UpdateEditorPermission updates an editor permission's respondent_access and field_filters.
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

// GetEditorResponseByID fetches one response for an editor, applying the permission's
// respondentAccess and field_filters checks.
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

// GetEditorAllowedRespondentByID fetches an editor allowed-respondent row by its ID.
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

// AddEditorAllowedRespondent adds one respondent to an editor's allowed list.
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

// RemoveEditorAllowedRespondent removes one respondent from an editor's allowed list.
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

// ListEditorAllowedRespondents returns every allowed respondent (joined with email/name).
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

// IsRespondentAllowedForEditor reports whether a given respondent is on the editor's allowed list.
func (s *Store) IsRespondentAllowedForEditor(ctx context.Context, permID, respondentID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM editor_allowed_respondents WHERE permission_id=$1 AND respondent_id=$2)`,
		permID, respondentID,
	).Scan(&exists)
	return exists, err
}

// ListEditorResponses returns the answers an editor may manage (restricted by respondentAccess
// and field_filters).
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
		"time":   "submitted_at",
		"waktu":  "submitted_at", // legacy value still sent by cached pages
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

// CountEditorResponses counts the answers an editor may manage.
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

// DeleteEditorPermission revokes an editor's access to a form.
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

// HasEditorFormPermission reports whether an editor has management access to a given form.
func (s *Store) HasEditorFormPermission(ctx context.Context, editorID, formID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM editor_form_permissions WHERE editor_id=$1 AND form_id=$2)`,
		editorID, formID,
	).Scan(&exists)
	return exists, err
}

// matchesFieldFilters verifies that the answers satisfy every field_filters restriction (exact match).
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

// maskAnswers filters the JSONB answer keys so only permitted fields remain visible.
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

// apiKeyCols is the canonical column list for selecting one API key; its order must match
// scanAPIKey.
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

// CreateAPIKey stores a new API key. keyHash is the SHA-256 hex digest of the real key —
// the key itself never reaches the database.
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

// ListAPIKeysByForm returns every API key for one form, along with its selected-respondent count.
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

// GetAPIKeyByID fetches a single API key by its ID.
func (s *Store) GetAPIKeyByID(ctx context.Context, id string) (*models.FormAPIKey, error) {
	k := &models.FormAPIKey{}
	err := scanAPIKey(k, s.pool.QueryRow(ctx, `SELECT `+apiKeyCols+` FROM form_api_keys WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return k, err
}

// GetAPIKeyByHash is the authentication lookup path: the key sent by the client is hashed
// and then found through the unique key_hash index.
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.FormAPIKey, error) {
	k := &models.FormAPIKey{}
	err := scanAPIKey(k, s.pool.QueryRow(ctx, `SELECT `+apiKeyCols+` FROM form_api_keys WHERE key_hash=$1`, keyHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return k, err
}

// UpdateAPIKey updates a key's label and all of its scope/security settings.
// key_hash never changes here — that is RotateAPIKey's job.
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

// RotateAPIKey replaces a key's credential without changing its scope. The old key stops
// working immediately because its key_hash is overwritten.
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

// DeleteAPIKey removes an API key along with its respondent list (ON DELETE CASCADE).
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

// TouchAPIKey records a key's most recent use. A failure here deliberately does not fail
// the request — the caller simply ignores the error.
func (s *Store) TouchAPIKey(ctx context.Context, id, ip string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE form_api_keys SET last_used_at=now(), last_used_ip=$2, request_count=request_count+1
		 WHERE id=$1`, id, ip)
	return err
}

// APIKeyScope builds a ResponseScope from an API key. Drafts are never included: the API
// shares submitted responses only.
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

// ListAPIKeyAllowedRespondents returns the allowed respondents (joined with email/name).
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

// AddAPIKeyAllowedRespondent adds one respondent to the allowed list.
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

// GetAPIKeyAllowedRespondentByID is used to verify ownership before deleting.
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

// RemoveAPIKeyAllowedRespondent removes one respondent from the allowed list.
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

// InsertAPIAccessLog records one /api/v1 call. It is also called for rejected requests —
// apiKeyID/formID may be nil when the key is unknown.
func (s *Store) InsertAPIAccessLog(ctx context.Context, l *models.APIAccessLog) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO api_access_logs(api_key_id,key_prefix,form_id,ip,path,query,status,row_count,error)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.APIKeyID, l.KeyPrefix, l.FormID, l.IP, l.Path, l.Query, l.Status, l.RowCount, l.Error)
	return err
}

// ListAPIAccessLogs returns one key's call history, newest first.
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

// PruneLogs deletes log rows older than the configured retention and returns the number of
// rows removed per table.
//
// The answer change history (response_revisions) is deliberately NOT pruned: it is part of
// the statistical data's provenance, not an operational log.
func (s *Store) PruneLogs(ctx context.Context, days int) (activity, apiAccess int64, err error) {
	if days <= 0 {
		return 0, 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	ct, err := s.pool.Exec(ctx, `DELETE FROM activity_logs WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, 0, err
	}
	activity = ct.RowsAffected()

	ct, err = s.pool.Exec(ctx, `DELETE FROM api_access_logs WHERE created_at < $1`, cutoff)
	if err != nil {
		return activity, 0, err
	}
	return activity, ct.RowsAffected(), nil
}

/* ---------------- answer change history ---------------- */

// InsertResponseRevision stores one answer-change record.
func (s *Store) InsertResponseRevision(ctx context.Context, rev *models.ResponseRevision) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO response_revisions(response_id,form_id,editor_id,editor_name,answers_before,answers_after,ip)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		rev.ResponseID, rev.FormID, rev.EditorID, rev.EditorName,
		rev.AnswersBefore, rev.AnswersAfter, rev.IP)
	return err
}

// ListResponseRevisions returns one answer's change history, newest first.
// The list of changed fields is computed here so the UI does not have to diff them itself.
func (s *Store) ListResponseRevisions(ctx context.Context, responseID string, limit int) ([]models.ResponseRevision, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id,response_id,form_id,editor_id,editor_name,answers_before,answers_after,ip,created_at
		 FROM response_revisions WHERE response_id=$1 ORDER BY created_at DESC LIMIT $2`, responseID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ResponseRevision
	for rows.Next() {
		rev := models.ResponseRevision{}
		if err := rows.Scan(&rev.ID, &rev.ResponseID, &rev.FormID, &rev.EditorID, &rev.EditorName,
			&rev.AnswersBefore, &rev.AnswersAfter, &rev.IP, &rev.CreatedAt); err != nil {
			return nil, err
		}
		rev.ChangedFields = ChangedAnswerFields(rev.AnswersBefore, rev.AnswersAfter)
		out = append(out, rev)
	}
	if out == nil {
		out = []models.ResponseRevision{}
	}
	return out, rows.Err()
}

/* ---------------- offline queue reports ---------------- */

// UpsertOfflineQueueReport records one device's latest queue state.
//
// A device reporting an empty queue deletes its row rather than storing zeros. The
// dashboard asks "whose device is holding data", so a clean device is not an answer
// to it — and keeping the rows around would bury the few that matter under a row per
// device per form forever.
func (s *Store) UpsertOfflineQueueReport(ctx context.Context, rep *models.OfflineQueueReport) error {
	if rep.Pending == 0 && rep.Failed == 0 && rep.Files == 0 {
		_, err := s.pool.Exec(ctx,
			`DELETE FROM offline_queue_reports
			 WHERE form_id=$1 AND respondent_id=$2 AND device_id=$3`,
			rep.FormID, rep.RespondentID, rep.DeviceID)
		return err
	}
	items := rep.Items
	if len(items) == 0 {
		items = json.RawMessage("[]")
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO offline_queue_reports
		   (form_id,respondent_id,device_id,pending,failed,files,oldest_queued_at,items,user_agent,reported_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
		 ON CONFLICT (form_id,respondent_id,device_id) DO UPDATE SET
		   pending=EXCLUDED.pending, failed=EXCLUDED.failed, files=EXCLUDED.files,
		   oldest_queued_at=EXCLUDED.oldest_queued_at, items=EXCLUDED.items,
		   user_agent=EXCLUDED.user_agent, reported_at=now()`,
		rep.FormID, rep.RespondentID, rep.DeviceID,
		rep.Pending, rep.Failed, rep.Files, rep.OldestQueuedAt, items, rep.UserAgent)
	return err
}

// ListOfflineQueueReports returns the devices still holding data for one form.
//
// Ordered by how bad it is rather than by time: rejected items first, because those
// will never send themselves, then by how long the oldest item has been waiting.
func (s *Store) ListOfflineQueueReports(ctx context.Context, formID string) ([]models.OfflineQueueReport, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT q.id,q.form_id,q.respondent_id,q.device_id,q.pending,q.failed,q.files,
		        q.oldest_queued_at,q.items,q.user_agent,q.reported_at,
		        COALESCE(r.name,''),COALESCE(r.email,'')
		 FROM offline_queue_reports q
		 LEFT JOIN respondents r ON r.id=q.respondent_id
		 WHERE q.form_id=$1
		 ORDER BY (q.failed>0) DESC, q.oldest_queued_at ASC NULLS LAST, q.reported_at DESC`, formID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.OfflineQueueReport{}
	for rows.Next() {
		rep := models.OfflineQueueReport{}
		if err := rows.Scan(&rep.ID, &rep.FormID, &rep.RespondentID, &rep.DeviceID,
			&rep.Pending, &rep.Failed, &rep.Files, &rep.OldestQueuedAt, &rep.Items,
			&rep.UserAgent, &rep.ReportedAt, &rep.RespondentName, &rep.RespondentEmail); err != nil {
			return nil, err
		}
		out = append(out, rep)
	}
	return out, rows.Err()
}

// PruneOfflineQueueReports drops reports nobody has refreshed in a long time.
//
// Note what this does NOT mean: a stale report is not proof the data was recovered,
// only that the device stopped talking to us. So the window is deliberately long —
// the row is the sole trace that work was stranded, and deleting it early would
// erase the evidence rather than the problem.
func (s *Store) PruneOfflineQueueReports(ctx context.Context, olderThan time.Duration) (int64, error) {
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM offline_queue_reports WHERE reported_at < now() - $1::interval`,
		olderThan.String())
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// ChangedAnswerFields compares two sets of answers and returns the names of the fields whose
// values differ (including those newly added or removed).
func ChangedAnswerFields(before, after json.RawMessage) []string {
	var a, b map[string]json.RawMessage
	_ = json.Unmarshal(before, &a)
	_ = json.Unmarshal(after, &b)
	seen := map[string]bool{}
	var out []string
	for k, av := range a {
		if bv, ok := b[k]; !ok || string(av) != string(bv) {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

/* ---------------- activity log ---------------- */

// InsertActivityLog records one admin action. A logging failure must not fail the action —
// the caller simply writes the error to stdout.
func (s *Store) InsertActivityLog(ctx context.Context, l *models.ActivityLog) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO activity_logs(actor_id,actor_name,actor_role,action,target_type,target_id,target_label,form_id,ip,detail)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		l.ActorID, l.ActorName, l.ActorRole, l.Action, l.TargetType, l.TargetID, l.TargetLabel, l.FormID, l.IP, l.Detail)
	return err
}

// ListActivityLogs returns the action history, newest first. An empty formID means all forms.
func (s *Store) ListActivityLogs(ctx context.Context, formID string, limit, offset int) ([]models.ActivityLog, int64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	where, args := "", []any{}
	if formID != "" {
		where = " WHERE form_id=$1"
		args = append(args, formID)
	}

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM activity_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx,
		fmt.Sprintf(`SELECT id,actor_id,actor_name,actor_role,action,target_type,target_id,target_label,
		                    form_id,ip,detail,created_at
		             FROM activity_logs%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
			where, len(args)-1, len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.ActivityLog
	for rows.Next() {
		l := models.ActivityLog{}
		if err := rows.Scan(&l.ID, &l.ActorID, &l.ActorName, &l.ActorRole, &l.Action, &l.TargetType,
			&l.TargetID, &l.TargetLabel, &l.FormID, &l.IP, &l.Detail, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	if out == nil {
		out = []models.ActivityLog{}
	}
	return out, total, rows.Err()
}

/* ---------------- wilayah ---------------- */

// GetWilayahByParent returns the child regions of parentCode.
// When parentCode is empty it returns every province-level region (kode_parent IS NULL).
func (s *Store) GetWilayahByParent(ctx context.Context, parentCode string) ([]models.WilayahItem, error) {
	var rows pgx.Rows
	var err error
	if parentCode == "" {
		rows, err = s.pool.Query(ctx,
			`SELECT kode_wilayah, nama_wilayah FROM wilayah
			 WHERE kode_parent IS NULL ORDER BY kode_wilayah`)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT kode_wilayah, nama_wilayah FROM wilayah
			 WHERE kode_parent = $1 ORDER BY kode_wilayah`,
			parentCode)
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
		items = []models.WilayahItem{} // return [] rather than null
	}
	return items, rows.Err()
}
