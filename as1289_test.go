package main

import (
	"reflect"
	"testing"
)

func TestParseAS1289Active(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{"ap1|aa2|aa3|aa4|aa5|h0|ga|l64039", 0},
		{"aa1|aa2|ap3|aa4|aa5|h14074000|gp|l1", 2},
		{"aa1|aa2|aa3|aa4|ap5|h0|ga|l0", 4},
		{"aa1|aa2|aa3|aa4|aa5|h0|ga|l0", -1}, // nothing active
		{"", -1},
	}
	for _, c := range cases {
		if got := parseAS1289Active(c.body); got != c.want {
			t.Errorf("parseAS1289Active(%q) = %d, want %d", c.body, got, c.want)
		}
	}
}

func TestDisplayNames(t *testing.T) {
	got := displayNames([]string{"OCD", "EFHW"}, 5)
	want := []string{"OCD", "EFHW", "Ant 3", "Ant 4", "Ant 5"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("displayNames = %v, want %v", got, want)
	}

	// Blank/whitespace-only entries also fall back to the generic label.
	got = displayNames([]string{"", "  ", "Beam"}, 3)
	want = []string{"Ant 1", "Ant 2", "Beam"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("displayNames = %v, want %v", got, want)
	}
}
