package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nahrx/eform/internal/models"
)

/* The rules that are most dangerous if they change silently: which columns may be
   read and which rows may leave. The tests below lock both down without a database. */

func TestMaskAnswers(t *testing.T) {
	raw := json.RawMessage(`{"name":"Budi","nik":"3201","salary":"5000000"}`)

	t.Run("an empty list means every column is readable", func(t *testing.T) {
		got := maskAnswers(raw, nil)
		if string(got) != string(raw) {
			t.Fatalf("without restrictions it should be untouched, got %s", got)
		}
	})

	t.Run("only the permitted columns remain", func(t *testing.T) {
		got := maskAnswers(raw, []string{"name"})
		var m map[string]string
		if err := json.Unmarshal(got, &m); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if len(m) != 1 || m["name"] != "Budi" {
			t.Fatalf("should be only {name:Budi}, got %v", m)
		}
		if _, ada := m["nik"]; ada {
			t.Error("NIK leaked even though it is not in visibleFields")
		}
		if _, ada := m["gaji"]; ada {
			t.Error("salary leaked even though it is not in visibleFields")
		}
	})

	t.Run("a requested but absent column produces nothing", func(t *testing.T) {
		got := maskAnswers(raw, []string{"tidak_ada"})
		var m map[string]any
		_ = json.Unmarshal(got, &m)
		if len(m) != 0 {
			t.Fatalf("should be empty, got %v", m)
		}
	})
}

func TestResponseScopeClauses(t *testing.T) {
	t.Run("drafts are excluded when IncludeDrafts=false", func(t *testing.T) {
		sc := ResponseScope{FormID: "f1", RespondentAccess: "all"}
		clause, _ := sc.clauses(nil)
		if !strings.Contains(clause, "status='submitted'") {
			t.Fatalf("a scope without drafts must filter on status, got %q", clause)
		}
	})

	t.Run("a viewer may still see drafts", func(t *testing.T) {
		sc := ResponseScope{FormID: "f1", RespondentAccess: "all", IncludeDrafts: true}
		clause, _ := sc.clauses(nil)
		if strings.Contains(clause, "status='submitted'") {
			t.Fatalf("a scope including drafts must not filter on status, got %q", clause)
		}
	})

	t.Run("selected respondents are restricted through the correct allow table", func(t *testing.T) {
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "selected",
			PermissionID: "p1", AllowedTable: AllowedTableAPIKey,
		}
		clause, args := sc.clauses(nil)
		if !strings.Contains(clause, AllowedTableAPIKey) {
			t.Fatalf("the clause must name %s, got %q", AllowedTableAPIKey, clause)
		}
		if len(args) != 1 || args[0] != "p1" {
			t.Fatalf("the permission id must be a query argument, got %v", args)
		}
	})

	t.Run("an unknown table name closes every row rather than opening them", func(t *testing.T) {
		// If a new access path ever forgets to set AllowedTable, the result must be
		// that nothing is returned — not that everything is.
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "selected",
			PermissionID: "p1", AllowedTable: "tabel_ngawur",
		}
		clause, _ := sc.clauses(nil)
		if !strings.Contains(clause, "false") {
			t.Fatalf("an unknown table must deny outright, got %q", clause)
		}
		if strings.Contains(clause, "tabel_ngawur") {
			t.Error("an unknown table name must never reach the SQL")
		}
	})

	t.Run("field-value filters become bound parameters", func(t *testing.T) {
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "all",
			FieldFilters: map[string]string{"kabupaten": "6472"},
		}
		clause, args := sc.clauses(nil)
		if !strings.Contains(clause, "answers->>'kabupaten'") {
			t.Fatalf("the clause must filter the field, got %q", clause)
		}
		if len(args) != 1 || args[0] != "6472" {
			t.Fatalf("the filter value must travel as an argument, got %v", args)
		}
		if strings.Contains(clause, "6472") {
			t.Error("the filter value must never be inlined into the SQL")
		}
	})

	t.Run("dangerous field names are ignored", func(t *testing.T) {
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "all",
			FieldFilters: map[string]string{"a'; DROP TABLE users; --": "x"},
		}
		clause, args := sc.clauses(nil)
		if strings.Contains(clause, "DROP TABLE") {
			t.Fatalf("an unsafe field name leaked into the SQL: %q", clause)
		}
		if len(args) != 0 {
			t.Fatalf("unsafe filters should be dropped, got %v", args)
		}
	})
}

func TestAPIKeyScopeNeverIncludesDrafts(t *testing.T) {
	// Drafts are half-finished entries; the API must not share them.
	sc := APIKeyScope(&models.FormAPIKey{ID: "k1", FormID: "f1", RespondentAccess: "selected"})
	if sc.IncludeDrafts {
		t.Fatal("an API key scope must not include drafts")
	}
	if sc.AllowedTable != AllowedTableAPIKey {
		t.Fatalf("an API key scope must use %s, got %s", AllowedTableAPIKey, sc.AllowedTable)
	}
}

func TestIsSafeIdentifier(t *testing.T) {
	safe := []string{"name", "kabupaten_kota", "f1", "A_9"}
	for _, s := range safe {
		if !isSafeIdentifier(s) {
			t.Errorf("%q should be considered safe", s)
		}
	}
	bahaya := []string{"", "a b", "a'b", "a;b", "a-b", "a.b", "a#b", strings.Repeat("a", 65)}
	for _, s := range bahaya {
		if isSafeIdentifier(s) {
			t.Errorf("%q should be rejected", s)
		}
	}
}
