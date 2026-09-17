package store

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

/* Answer columns in the order the instrument asks the questions.

   The column set comes out of the database as the distinct keys found in the answers,
   and a DISTINCT list has no order but the alphabetical one — which puts "10.2"
   before "2.1" and scatters a roster's rows between unrelated questions. An export
   read by a person, or loaded into a spreadsheet next to the questionnaire, wants
   the questions in the order they were asked.

   Keys are built as the form builds them: a field's dataKey, or for a roster row
   "<roster>#<row>#<field>", nested as deep as the rosters go, plus "<roster>#count".
   Each key is turned into a rank — the position of every name on its path within its
   parent, with the row index in between — and the columns are sorted by that rank.
   Keys the instrument no longer has (a field since deleted) keep their data and go
   last, alphabetically. */

type schemaNode struct {
	pos      int
	children map[string]*schemaNode
}

func buildSchemaTree(schema json.RawMessage) *schemaNode {
	root := &schemaNode{children: map[string]*schemaNode{}}
	if len(schema) == 0 {
		return root
	}
	var doc struct {
		Pages []json.RawMessage `json:"pages"`
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return root
	}
	pos := 0
	for _, p := range doc.Pages {
		walkComponents(p, root, &pos)
	}
	return root
}

// walkComponents numbers every named field and roster in document order. Blocks and
// sections are transparent — their children count as if they sat in the parent — so
// the numbering follows the questions, not the layout boxes around them. A roster
// starts a nested numbering of its own.
func walkComponents(raw json.RawMessage, parent *schemaNode, pos *int) {
	var c struct {
		Kind       string            `json:"kind"`
		Name       string            `json:"name"`
		Components []json.RawMessage `json:"components"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return
	}
	switch c.Kind {
	case "field":
		if c.Name != "" {
			if _, dup := parent.children[c.Name]; !dup {
				parent.children[c.Name] = &schemaNode{pos: *pos}
				*pos++
			}
		}
	case "roster":
		if c.Name == "" {
			return
		}
		n, dup := parent.children[c.Name]
		if !dup {
			n = &schemaNode{pos: *pos, children: map[string]*schemaNode{}}
			parent.children[c.Name] = n
			*pos++
		}
		inner := 0
		for _, ch := range c.Components {
			walkComponents(ch, n, &inner)
		}
	default: // page, block, section — transparent
		for _, ch := range c.Components {
			walkComponents(ch, parent, pos)
		}
	}
}

// rank turns one column key into a comparable path; known is false when any name on
// the key is not in the instrument.
func (n *schemaNode) rank(key string) (path []int, known bool) {
	parts := strings.Split(key, "#")
	cur := n
	for i := 0; i < len(parts); i++ {
		name := parts[i]
		child, ok := cur.children[name]
		if !ok {
			return path, false
		}
		path = append(path, child.pos)
		if child.children == nil { // a field: nothing may follow it
			return path, i == len(parts)-1
		}
		// a roster: the row index comes next, then a name inside the roster
		if i+1 >= len(parts) {
			return path, false
		}
		if parts[i+1] == "count" && i+1 == len(parts)-1 {
			// "<roster>#count" — the row count sits ahead of the rows it counts.
			path = append(path, -1)
			return path, true
		}
		idx, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return path, false
		}
		path = append(path, idx)
		cur = child
		i++
	}
	return path, false
}

// OrderColumnsBySchema sorts answer keys into instrument order; see the note above.
func OrderColumnsBySchema(schema json.RawMessage, cols []string) []string {
	tree := buildSchemaTree(schema)
	type ranked struct {
		key   string
		path  []int
		known bool
	}
	rs := make([]ranked, len(cols))
	for i, c := range cols {
		p, ok := tree.rank(c)
		rs[i] = ranked{c, p, ok}
	}
	sort.SliceStable(rs, func(a, b int) bool {
		ra, rb := rs[a], rs[b]
		if ra.known != rb.known {
			return ra.known // known columns first
		}
		if !ra.known {
			return ra.key < rb.key
		}
		for i := 0; i < len(ra.path) && i < len(rb.path); i++ {
			if ra.path[i] != rb.path[i] {
				return ra.path[i] < rb.path[i]
			}
		}
		if len(ra.path) != len(rb.path) {
			return len(ra.path) < len(rb.path)
		}
		return ra.key < rb.key
	})
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.key
	}
	return out
}
