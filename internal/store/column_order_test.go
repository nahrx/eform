package store

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOrderColumnsBySchema(t *testing.T) {
	schema := json.RawMessage(`{"pages":[
	  {"kind":"page","components":[
	    {"kind":"block","components":[
	      {"kind":"field","name":"nama"},
	      {"kind":"field","name":"10.2"},
	      {"kind":"section","components":[{"kind":"field","name":"2.1"}]},
	      {"kind":"roster","name":"art","components":[
	        {"kind":"field","name":"1.6"},
	        {"kind":"roster","name":"usaha","components":[{"kind":"field","name":"u1"}]},
	        {"kind":"field","name":"1.8"}
	      ]},
	      {"kind":"field","name":"akhir"}
	    ]}
	  ]},
	  {"kind":"page","components":[{"kind":"field","name":"catatan"}]}
	]}`)
	// Alphabetical, as the database hands them over.
	cols := []string{
		"10.2", "2.1", "akhir", "art#0#1.6", "art#0#1.8", "art#0#usaha#0#u1", "art#0#usaha#count",
		"art#1#1.6", "art#10#1.6", "art#2#1.6", "art#count", "catatan", "nama", "zz_removed",
	}
	want := []string{
		"nama", "10.2", "2.1",
		"art#count",
		"art#0#1.6", "art#0#usaha#count", "art#0#usaha#0#u1", "art#0#1.8",
		"art#1#1.6", "art#2#1.6", "art#10#1.6",
		"akhir", "catatan",
		"zz_removed",
	}
	got := OrderColumnsBySchema(schema, cols)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("\n got %v\nwant %v", got, want)
	}
}

func TestOrderColumnsBySchemaNoSchema(t *testing.T) {
	got := OrderColumnsBySchema(nil, []string{"b", "a"})
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("got %v", got)
	}
}
