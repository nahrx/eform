package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bpskaltim/eform-backend/internal/models"
)

/* Aturan yang paling berbahaya kalau diam-diam berubah: kolom mana yang boleh terbaca
   dan baris mana yang boleh keluar. Tes di bawah mengunci keduanya tanpa perlu database. */

func TestMaskAnswers(t *testing.T) {
	raw := json.RawMessage(`{"nama":"Budi","nik":"3201","gaji":"5000000"}`)

	t.Run("daftar kosong berarti semua kolom terbaca", func(t *testing.T) {
		got := maskAnswers(raw, nil)
		if string(got) != string(raw) {
			t.Fatalf("tanpa pembatasan seharusnya utuh, dapat %s", got)
		}
	})

	t.Run("hanya kolom yang diizinkan yang tersisa", func(t *testing.T) {
		got := maskAnswers(raw, []string{"nama"})
		var m map[string]string
		if err := json.Unmarshal(got, &m); err != nil {
			t.Fatalf("hasil bukan JSON valid: %v", err)
		}
		if len(m) != 1 || m["nama"] != "Budi" {
			t.Fatalf("harusnya hanya {nama:Budi}, dapat %v", m)
		}
		if _, ada := m["nik"]; ada {
			t.Error("NIK bocor padahal tidak masuk visibleFields")
		}
		if _, ada := m["gaji"]; ada {
			t.Error("gaji bocor padahal tidak masuk visibleFields")
		}
	})

	t.Run("kolom yang diminta tapi tidak ada tidak memunculkan apa pun", func(t *testing.T) {
		got := maskAnswers(raw, []string{"tidak_ada"})
		var m map[string]any
		_ = json.Unmarshal(got, &m)
		if len(m) != 0 {
			t.Fatalf("harusnya kosong, dapat %v", m)
		}
	})
}

func TestResponseScopeClauses(t *testing.T) {
	t.Run("draft ditutup saat IncludeDrafts=false", func(t *testing.T) {
		sc := ResponseScope{FormID: "f1", RespondentAccess: "all"}
		clause, _ := sc.clauses(nil)
		if !strings.Contains(clause, "status='submitted'") {
			t.Fatalf("scope tanpa draft harus memfilter status, dapat %q", clause)
		}
	})

	t.Run("viewer tetap boleh melihat draft", func(t *testing.T) {
		sc := ResponseScope{FormID: "f1", RespondentAccess: "all", IncludeDrafts: true}
		clause, _ := sc.clauses(nil)
		if strings.Contains(clause, "status='submitted'") {
			t.Fatalf("scope dengan draft tidak boleh memfilter status, dapat %q", clause)
		}
	})

	t.Run("responden terpilih dibatasi lewat tabel izin yang benar", func(t *testing.T) {
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "selected",
			PermissionID: "p1", AllowedTable: AllowedTableAPIKey,
		}
		clause, args := sc.clauses(nil)
		if !strings.Contains(clause, AllowedTableAPIKey) {
			t.Fatalf("klausa harus menyebut %s, dapat %q", AllowedTableAPIKey, clause)
		}
		if len(args) != 1 || args[0] != "p1" {
			t.Fatalf("permission id harus jadi argumen query, dapat %v", args)
		}
	})

	t.Run("nama tabel tak dikenal menutup semua baris, bukan membukanya", func(t *testing.T) {
		// Kalau suatu saat ada jalur akses baru yang lupa mengisi AllowedTable,
		// hasilnya harus tidak mengembalikan apa pun — bukan mengembalikan semuanya.
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "selected",
			PermissionID: "p1", AllowedTable: "tabel_ngawur",
		}
		clause, _ := sc.clauses(nil)
		if !strings.Contains(clause, "false") {
			t.Fatalf("tabel tak dikenal harus menutup total, dapat %q", clause)
		}
		if strings.Contains(clause, "tabel_ngawur") {
			t.Error("nama tabel tak dikenal tidak boleh ikut masuk SQL")
		}
	})

	t.Run("filter nilai variabel jadi argumen berparameter", func(t *testing.T) {
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "all",
			FieldFilters: map[string]string{"kabupaten": "6472"},
		}
		clause, args := sc.clauses(nil)
		if !strings.Contains(clause, "answers->>'kabupaten'") {
			t.Fatalf("klausa harus menyaring field, dapat %q", clause)
		}
		if len(args) != 1 || args[0] != "6472" {
			t.Fatalf("nilai filter harus lewat argumen, dapat %v", args)
		}
		if strings.Contains(clause, "6472") {
			t.Error("nilai filter tidak boleh ditanam langsung di SQL")
		}
	})

	t.Run("nama variabel berbahaya diabaikan", func(t *testing.T) {
		sc := ResponseScope{
			FormID: "f1", RespondentAccess: "all",
			FieldFilters: map[string]string{"a'; DROP TABLE users; --": "x"},
		}
		clause, args := sc.clauses(nil)
		if strings.Contains(clause, "DROP TABLE") {
			t.Fatalf("nama field tidak aman bocor ke SQL: %q", clause)
		}
		if len(args) != 0 {
			t.Fatalf("filter tidak aman seharusnya dibuang, dapat %v", args)
		}
	})
}

func TestAPIKeyScopeTidakPernahIkutkanDraft(t *testing.T) {
	// Draft adalah isian setengah jadi; API tidak boleh membagikannya.
	sc := APIKeyScope(&models.FormAPIKey{ID: "k1", FormID: "f1", RespondentAccess: "selected"})
	if sc.IncludeDrafts {
		t.Fatal("scope API key tidak boleh menyertakan draft")
	}
	if sc.AllowedTable != AllowedTableAPIKey {
		t.Fatalf("scope API key harus memakai %s, dapat %s", AllowedTableAPIKey, sc.AllowedTable)
	}
}

func TestIsSafeIdentifier(t *testing.T) {
	aman := []string{"nama", "kabupaten_kota", "f1", "A_9"}
	for _, s := range aman {
		if !isSafeIdentifier(s) {
			t.Errorf("%q seharusnya dianggap aman", s)
		}
	}
	bahaya := []string{"", "a b", "a'b", "a;b", "a-b", "a.b", "a#b", strings.Repeat("a", 65)}
	for _, s := range bahaya {
		if isSafeIdentifier(s) {
			t.Errorf("%q seharusnya ditolak", s)
		}
	}
}
