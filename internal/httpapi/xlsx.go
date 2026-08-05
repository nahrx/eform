package httpapi

import (
	"archive/zip"
	"fmt"
	"io"
	"regexp"
	"strings"
)

/* Penulis XLSX minimal untuk ekspor jawaban.

   Sengaja ditulis sendiri, bukan memakai pustaka spreadsheet, karena yang dibutuhkan
   hanya satu lembar berisi teks dan angka — sementara pustaka semacam itu menarik
   banyak dependensi transitif ke dalam sistem yang memuat data rahasia.

   Bentuknya streaming: baris ditulis langsung ke dalam entri zip di response writer,
   jadi ekspor puluhan ribu jawaban tidak perlu ditampung di memori lebih dulu.
   Semua teks memakai inlineStr sehingga tidak perlu tabel sharedStrings. */

type xlsxWriter struct {
	zw    *zip.Writer
	sheet io.Writer
	row   int
	err   error
}

const xlsxMaxCellLen = 32767 // batas Excel per sel

func newXLSXWriter(w io.Writer, sheetName string) (*xlsxWriter, error) {
	zw := zip.NewWriter(w)
	x := &xlsxWriter{zw: zw}

	if sheetName == "" {
		sheetName = "Jawaban"
	}
	// Excel menolak beberapa karakter pada nama lembar dan membatasi 31 karakter.
	sheetName = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:\/?*[]`, r) {
			return '-'
		}
		return r
	}, sheetName)
	if len([]rune(sheetName)) > 31 {
		sheetName = string([]rune(sheetName)[:31])
	}

	files := []struct{ name, body string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
			`</Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="` + xmlEscape(sheetName) + `" sheetId="1" r:id="rId1"/></sheets>` +
			`</workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`</Relationships>`},
	}
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, f.body); err != nil {
			return nil, err
		}
	}

	sheet, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	x.sheet = sheet
	// <dimension> sengaja tidak ditulis: ukurannya belum diketahui saat streaming,
	// dan elemen itu memang opsional.
	_, err = io.WriteString(sheet,
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	return x, err
}

// numericRe cocok untuk angka yang aman ditulis sebagai bilangan.
// Angka berawalan nol (mis. kode wilayah "0101") sengaja TIDAK cocok, karena kalau
// ditulis sebagai bilangan nol depannya hilang dan datanya rusak.
var numericRe = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?$`)

func isSpreadsheetNumber(v string) bool {
	if v == "" || len(v) > 15 {
		return false
	}
	return numericRe.MatchString(v)
}

// WriteRow menulis satu baris sel. Nilai yang berbentuk angka ditulis sebagai bilangan
// agar bisa langsung dijumlahkan di Excel; sisanya ditulis sebagai teks apa adanya.
func (x *xlsxWriter) WriteRow(cells []string) {
	if x.err != nil {
		return
	}
	x.row++
	var b strings.Builder
	fmt.Fprintf(&b, `<row r="%d">`, x.row)
	for i, c := range cells {
		ref := fmt.Sprintf("%s%d", columnName(i), x.row)
		if len([]rune(c)) > xlsxMaxCellLen {
			c = string([]rune(c)[:xlsxMaxCellLen])
		}
		if isSpreadsheetNumber(c) {
			fmt.Fprintf(&b, `<c r="%s"><v>%s</v></c>`, ref, c)
			continue
		}
		if c == "" {
			continue // sel kosong tidak perlu ditulis
		}
		fmt.Fprintf(&b, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, xmlEscape(c))
	}
	b.WriteString(`</row>`)
	_, x.err = io.WriteString(x.sheet, b.String())
}

func (x *xlsxWriter) Close() error {
	if x.err != nil {
		x.zw.Close()
		return x.err
	}
	if _, err := io.WriteString(x.sheet, `</sheetData></worksheet>`); err != nil {
		x.zw.Close()
		return err
	}
	return x.zw.Close()
}

// columnName mengubah indeks kolom (0-based) jadi huruf ala Excel: A, B, ... Z, AA, AB, ...
func columnName(i int) string {
	name := ""
	for i >= 0 {
		name = string(rune('A'+i%26)) + name
		i = i/26 - 1
	}
	return name
}

// xmlEscape membuang karakter kontrol yang tidak sah di XML 1.0 lalu meng-escape
// karakter yang punya arti khusus. Tanpa pembuangan itu, satu karakter kontrol dalam
// jawaban bisa membuat seluruh berkas ditolak Excel.
func xmlEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		case '\t', '\n', '\r':
			b.WriteRune(r)
		default:
			if r < 0x20 || (r >= 0xD800 && r <= 0xDFFF) || r == 0xFFFE || r == 0xFFFF {
				continue // tidak sah di XML 1.0 — buang
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
