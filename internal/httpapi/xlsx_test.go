package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"
)

func TestColumnName(t *testing.T) {
	cases := map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA", 701: "ZZ", 702: "AAA"}
	for i, want := range cases {
		if got := columnName(i); got != want {
			t.Errorf("columnName(%d) = %q, mau %q", i, got, want)
		}
	}
}

func TestIsSpreadsheetNumber(t *testing.T) {
	angka := []string{"0", "2", "-5", "6472", "3.14", "-0.5"}
	for _, v := range angka {
		if !isSpreadsheetNumber(v) {
			t.Errorf("%q seharusnya dianggap angka", v)
		}
	}
	// Yang paling penting: kode berawalan nol harus tetap teks, kalau tidak
	// "0101" berubah jadi 101 dan datanya rusak.
	teks := []string{"", "0101", "007", "6472A", "1,5", "1e5", "abc", "1234567890123456", " 5"}
	for _, v := range teks {
		if isSpreadsheetNumber(v) {
			t.Errorf("%q seharusnya tetap teks", v)
		}
	}
}

func TestXMLEscapeMembuangKarakterKontrol(t *testing.T) {
	got := xmlEscape("a\x00b\x07c")
	if strings.ContainsAny(got, "\x00\x07") {
		t.Fatalf("karakter kontrol masih ada: %q", got)
	}
	if got != "abc" {
		t.Fatalf("dapat %q", got)
	}
	if e := xmlEscape(`a<b>&"c'`); e != `a&lt;b&gt;&amp;&quot;c&apos;` {
		t.Fatalf("escape salah: %q", e)
	}
	// tab/newline sah di XML dan harus dipertahankan
	if xmlEscape("a\nb\tc") != "a\nb\tc" {
		t.Error("tab/newline seharusnya dipertahankan")
	}
}

// TestXLSXStrukturValid memastikan berkas yang dihasilkan benar-benar zip berisi
// bagian-bagian wajib OOXML dan XML-nya well-formed.
func TestXLSXStrukturValid(t *testing.T) {
	var buf bytes.Buffer
	x, err := newXLSXWriter(&buf, "Jawaban")
	if err != nil {
		t.Fatal(err)
	}
	x.WriteRow([]string{"id", "nama", "jumlah", "kode"})
	x.WriteRow([]string{"r1", "Budi <&>", "2", "0101"})
	x.WriteRow([]string{"r2", "Ani \"Q\"", "3.5", "6472"})
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("bukan zip yang sah: %v", err)
	}
	wajib := map[string]bool{
		"[Content_Types].xml":        false,
		"_rels/.rels":                false,
		"xl/workbook.xml":            false,
		"xl/_rels/workbook.xml.rels": false,
		"xl/worksheets/sheet1.xml":   false,
	}
	for _, f := range zr.File {
		if _, ada := wajib[f.Name]; ada {
			wajib[f.Name] = true
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		// Setiap bagian harus XML yang well-formed.
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s bukan XML valid: %v", f.Name, err)
			}
		}
		if f.Name == "xl/worksheets/sheet1.xml" {
			s := string(data)
			if !strings.Contains(s, `<v>2</v>`) {
				t.Error("angka seharusnya ditulis sebagai bilangan")
			}
			if !strings.Contains(s, `0101`) || strings.Contains(s, `<v>0101</v>`) {
				t.Error("kode berawalan nol harus tetap teks")
			}
			if !strings.Contains(s, "Budi &lt;&amp;&gt;") {
				t.Error("karakter khusus harus di-escape")
			}
		}
	}
	for name, ada := range wajib {
		if !ada {
			t.Errorf("bagian wajib hilang: %s", name)
		}
	}
}

func TestXLSXNamaLembarDibersihkan(t *testing.T) {
	var buf bytes.Buffer
	x, _ := newXLSXWriter(&buf, "Lap/oran: 2026 [uji] yang namanya sangat panjang sekali")
	x.WriteRow([]string{"a"})
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	for _, f := range zr.File {
		if f.Name != "xl/workbook.xml" {
			continue
		}
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		s := string(data)
		// Ambil isi atribut name="..." saja — bagian lain wajar memuat ':' (mis. r:id).
		start := strings.Index(s, `<sheet name="`) + len(`<sheet name="`)
		name := s[start : start+strings.Index(s[start:], `"`)]
		if strings.ContainsAny(name, `:\/?*[]`) {
			t.Errorf("nama lembar masih memuat karakter terlarang: %q", name)
		}
		if len([]rune(name)) > 31 {
			t.Errorf("nama lembar melebihi 31 karakter: %q", name)
		}
	}
}

// TestXLSXTulisKeBerkas menulis contoh berkas ke path di env XLSX_OUT supaya bisa
// diverifikasi pembaca spreadsheet sungguhan di luar Go. Dilewati kalau env tak diisi.
func TestXLSXTulisKeBerkas(t *testing.T) {
	out := os.Getenv("XLSX_OUT")
	if out == "" {
		t.Skip("XLSX_OUT tidak diisi")
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	x, err := newXLSXWriter(f, "Jawaban SE2026")
	if err != nil {
		t.Fatal(err)
	}
	x.WriteRow([]string{"id", "nama", "kode_wilayah", "jumlah", "catatan", "panjang"})
	x.WriteRow([]string{"r1", `Budi <Ámir> & "Co"`, "0101", "2", "baris\nbaru\tdan tab", strings.Repeat("x", 40000)})
	x.WriteRow([]string{"r2", "Ani ✓ émoji 😀", "6472", "3.5", "", ""})
	x.WriteRow([]string{"r3", "kontrol\x01\x02", "007", "-5", "kosong di tengah", "akhir"})
	wide := make([]string, 30)
	for i := range wide {
		wide[i] = "k" + string(rune('a'+i%26))
	}
	x.WriteRow(wide)
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}
}
