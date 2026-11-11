// Package icons holds the vendored Tabler icons. The embedded files are the
// only list of valid icon names: a name is valid when its .svg file exists.
package icons

import (
	"embed"
	"regexp"
	"sort"
	"strings"
)

// Fallback is drawn for an icon widget whose value matches no variant.
const Fallback = "square-rotated"

//go:embed svg/*.svg
var files embed.FS

var validName = regexp.MustCompile(`^[a-z0-9-]+$`)

// Has reports whether an icon with this name is vendored.
func Has(name string) bool {
	_, err := Read(name)
	return err == nil
}

// Read returns the SVG of a vendored icon.
func Read(name string) ([]byte, error) {
	if !validName.MatchString(name) {
		return nil, &notFoundError{name}
	}
	return files.ReadFile("svg/" + name + ".svg")
}

// Names lists every vendored icon name in alphabetical order.
func Names() []string {
	entries, _ := files.ReadDir("svg")
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, strings.TrimSuffix(entry.Name(), ".svg"))
	}
	sort.Strings(names)
	return names
}

type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return "icon " + e.name + " not found" }
