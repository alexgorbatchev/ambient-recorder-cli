// Package microphone parses device preferences shared by configuration and capture.
package microphone

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Selector matches a microphone name pattern or an exact stable UID.
type Selector struct {
	Exclude bool
	uid     bool
	value   string
	pattern *regexp.Regexp
}

// Parse compiles name wildcards once, outside inventory and capture loops.
func Parse(preferences []string) ([]Selector, error) {
	selectors := make([]Selector, 0, len(preferences))
	for _, raw := range preferences {
		s, err := parse(raw)
		if err != nil {
			return nil, fmt.Errorf("microphone preference %q: %w", raw, err)
		}
		selectors = append(selectors, s)
	}
	return selectors, nil
}

func parse(raw string) (Selector, error) {
	value, exclude := strings.CutPrefix(raw, "!")
	value, uid := strings.CutPrefix(value, "uid:")
	s := Selector{Exclude: exclude, uid: uid, value: value}
	if strings.TrimSpace(value) == "" {
		return Selector{}, errors.New("provide a name pattern or uid:<UID>, optionally prefixed with !")
	}
	if uid || value == "*" || !strings.Contains(value, "*") {
		return s, nil
	}
	// Names are text, not paths: '*' also matches slashes and newlines. Quoting
	// every literal part keeps device punctuation from becoming regexp syntax.
	parts := strings.Split(value, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	pattern, err := regexp.Compile(`(?s)\A` + strings.Join(parts, ".*") + `\z`)
	if err != nil {
		return Selector{}, fmt.Errorf("compile name pattern: %w", err)
	}
	s.pattern = pattern
	return s, nil
}

// Match compares a device with this selector independently of its exclusion flag.
func (s Selector) Match(name, uid string) bool {
	if s.uid {
		return s.value == uid
	}
	if s.pattern != nil {
		return s.pattern.MatchString(name)
	}
	return s.value == "*" || s.value == name
}
