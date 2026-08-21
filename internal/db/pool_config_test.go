package db

import (
	"os"
	"testing"
)

// The pool limit is the ceiling on how many requests can touch the database at once,
// so the defaults and the override are both worth pinning down.
func TestEnvInt(t *testing.T) {
	const key = "DB_MAX_CONNS_TEST_ONLY"

	cases := []struct {
		set  string
		def  int32
		want int32
		why  string
	}{
		{"", 25, 25, "unset falls back to the default"},
		{"40", 25, 40, "a valid number wins"},
		{"  40  ", 25, 40, "surrounding spaces are tolerated"},
		{"0", 25, 25, "zero would deadlock the pool"},
		{"-5", 25, 25, "negative is meaningless"},
		{"banyak", 25, 25, "non-numeric falls back rather than crashing"},
	}
	for _, c := range cases {
		if c.set == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, c.set)
		}
		if got := envInt(key, c.def); got != c.want {
			t.Errorf("%s: envInt(%q, %d) = %d, want %d", c.why, c.set, c.def, got, c.want)
		}
	}
	os.Unsetenv(key)
}
