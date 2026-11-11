package icons

import (
	"os"
	"strings"
	"testing"
)

// makefileIcons reads TABLER_ICONS, which `make vendor` downloads.
func makefileIcons(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	collecting := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "TABLER_ICONS :=") {
			collecting = true
			line = strings.TrimPrefix(line, "TABLER_ICONS :=")
		} else if !collecting {
			continue
		}
		continued := strings.HasSuffix(line, "\\")
		names = append(names, strings.Fields(strings.TrimSuffix(line, "\\"))...)
		if !continued {
			break
		}
	}
	return names
}

// An icon added to the Makefile without running `make vendor`, or the other
// way round, fails here.
func TestVendoredIconsMatchMakefile(t *testing.T) {
	want := makefileIcons(t)
	if len(want) == 0 {
		t.Fatal("no TABLER_ICONS found in the Makefile")
	}
	have := make(map[string]bool)
	for _, name := range Names() {
		have[name] = true
	}
	for _, name := range want {
		if !have[name] {
			t.Errorf("%s is listed in the Makefile but not vendored: run make vendor", name)
		}
		delete(have, name)
	}
	for name := range have {
		t.Errorf("%s is vendored but missing from TABLER_ICONS in the Makefile", name)
	}
}

func TestFallbackExistsAndNamesAreChecked(t *testing.T) {
	if !Has(Fallback) {
		t.Fatalf("fallback icon %q is not vendored", Fallback)
	}
	for _, name := range []string{"", "nope", "../icons", "bug.svg", "Bug"} {
		if Has(name) {
			t.Errorf("%q must not be a valid icon name", name)
		}
	}
}
