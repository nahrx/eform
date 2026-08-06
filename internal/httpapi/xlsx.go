package httpapi

import (
	"archive/zip"
	"fmt"
	"io"
	"regexp"
	"strings"
)

/* A minimal XLSX writer for exporting responses.

   Deliberately hand-written rather than using a spreadsheet library, because all that
   is needed is a single sheet of text and numbers — while such a library would pull a
   long tail of transitive dependencies into a system that holds confidential data.

   It streams: rows are written straight into a zip entry on the response writer, so
   exporting tens of thousands of responses never has to be buffered in memory first.
   All text uses inlineStr, which removes the need for a sharedStrings table. */

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
		sheetName = "Responses"
	}
	// Excel rejects certain characters in sheet names and caps them at 31 characters.
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
	// <dimension> is deliberately omitted: the extent is unknown while streaming,
	// and the element is optional anyway.
	_, err = io.WriteString(sheet,
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	return x, err
}

// numericRe matches numbers that are safe to write as numeric cells.
// Leading-zero numbers (region codes such as "0101") deliberately do NOT match, because
// writing them as numbers would drop the leading zero and corrupt the data.
var numericRe = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?$`)

func isSpreadsheetNumber(v string) bool {
	if v == "" || len(v) > 15 {
		return false
	}
	return numericRe.MatchString(v)
}

// WriteRow writes one row of cells. Numeric-looking values are written as numbers so they
// can be summed directly in Excel; everything else is written as text, verbatim.
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
			continue // empty cells need not be written
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

// columnName turns a 0-based column index into Excel-style letters: A, B, ... Z, AA, AB, ...
func columnName(i int) string {
	name := ""
	for i >= 0 {
		name = string(rune('A'+i%26)) + name
		i = i/26 - 1
	}
	return name
}

// xmlEscape strips control characters that are illegal in XML 1.0 and then escapes the
// characters with special meaning. Without that stripping, a single control character in
// an answer could make Excel reject the whole file.
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
				continue // illegal in XML 1.0 — drop it
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
