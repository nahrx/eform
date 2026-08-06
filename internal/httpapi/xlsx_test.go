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
	numbers := []string{"0", "2", "-5", "6472", "3.14", "-0.5"}
	for _, v := range numbers {
		if !isSpreadsheetNumber(v) {
			t.Errorf("%q should be treated as a number", v)
		}
	}
	// Most important of all: leading-zero codes must stay text, otherwise
	// "0101" would become 101 and the data would be corrupt.
	texts := []string{"", "0101", "007", "6472A", "1,5", "1e5", "abc", "1234567890123456", " 5"}
	for _, v := range texts {
		if isSpreadsheetNumber(v) {
			t.Errorf("%q should stay text", v)
		}
	}
}

func TestXMLEscapeStripsControlCharacters(t *testing.T) {
	got := xmlEscape("a\x00b\x07c")
	if strings.ContainsAny(got, "\x00\x07") {
		t.Fatalf("control characters remain: %q", got)
	}
	if got != "abc" {
		t.Fatalf("got %q", got)
	}
	if e := xmlEscape(`a<b>&"c'`); e != `a&lt;b&gt;&amp;&quot;c&apos;` {
		t.Fatalf("wrong escaping: %q", e)
	}
	// tab/newline are legal in XML and must be preserved
	if xmlEscape("a\nb\tc") != "a\nb\tc" {
		t.Error("tab/newline should be preserved")
	}
}

// TestXLSXStructureIsValid checks that the produced file really is a zip containing
// the required OOXML parts, and that its XML is well-formed.
func TestXLSXStructureIsValid(t *testing.T) {
	var buf bytes.Buffer
	x, err := newXLSXWriter(&buf, "Responses")
	if err != nil {
		t.Fatal(err)
	}
	x.WriteRow([]string{"id", "name", "amount", "code"})
	x.WriteRow([]string{"r1", "Budi <&>", "2", "0101"})
	x.WriteRow([]string{"r2", "Ani \"Q\"", "3.5", "6472"})
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
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
		// Every part must be well-formed XML.
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s is not valid XML: %v", f.Name, err)
			}
		}
		if f.Name == "xl/worksheets/sheet1.xml" {
			s := string(data)
			if !strings.Contains(s, `<v>2</v>`) {
				t.Error("a number should be written as numeric")
			}
			if !strings.Contains(s, `0101`) || strings.Contains(s, `<v>0101</v>`) {
				t.Error("a leading-zero code must stay text")
			}
			if !strings.Contains(s, "Budi &lt;&amp;&gt;") {
				t.Error("special characters must be escaped")
			}
		}
	}
	for name, ada := range wajib {
		if !ada {
			t.Errorf("required part missing: %s", name)
		}
	}
}

func TestXLSXSheetNameIsSanitised(t *testing.T) {
	var buf bytes.Buffer
	x, _ := newXLSXWriter(&buf, "Rep/ort: 2026 [test] with an extremely long name")
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
		// Only take the name="..." attribute — other parts legitimately contain ':' (r:id, say).
		start := strings.Index(s, `<sheet name="`) + len(`<sheet name="`)
		name := s[start : start+strings.Index(s[start:], `"`)]
		if strings.ContainsAny(name, `:\/?*[]`) {
			t.Errorf("the sheet name still contains a forbidden character: %q", name)
		}
		if len([]rune(name)) > 31 {
			t.Errorf("the sheet name exceeds 31 characters: %q", name)
		}
	}
}

// TestXLSXWriteToFile writes a sample file to the path in XLSX_OUT so it can be
// verified by a real spreadsheet reader outside Go. Skipped when the env var is unset.
func TestXLSXWriteToFile(t *testing.T) {
	out := os.Getenv("XLSX_OUT")
	if out == "" {
		t.Skip("XLSX_OUT is not set")
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	x, err := newXLSXWriter(f, "Responses SE2026")
	if err != nil {
		t.Fatal(err)
	}
	x.WriteRow([]string{"id", "name", "kode_wilayah", "amount", "note", "long"})
	x.WriteRow([]string{"r1", `Budi <Ámir> & "Co"`, "0101", "2", "line\nbreak\tand tab", strings.Repeat("x", 40000)})
	x.WriteRow([]string{"r2", "Ani ✓ émoji 😀", "6472", "3.5", "", ""})
	x.WriteRow([]string{"r3", "control\x01\x02", "007", "-5", "empty in the middle", "end"})
	wide := make([]string, 30)
	for i := range wide {
		wide[i] = "k" + string(rune('a'+i%26))
	}
	x.WriteRow(wide)
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}
}
