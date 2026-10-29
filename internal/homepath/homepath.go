// Package homepath formats filesystem paths for display by folding the user's
// home directory to "~". Folding is for output only: filesystem calls,
// subprocess arguments and written files keep absolute paths.
package homepath

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Fold returns path with a leading home directory shown as "~". The home
// directory itself becomes "~" and paths beneath it become "~/<rest>". Only whole
// path segments match, so a sibling such as /home/developer is unchanged when
// home is /home/dev. Paths outside home, relative paths, and every path when
// the home directory is unknown are returned unchanged.
func Fold(path string) string {
	home := homeDir()
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, found := strings.CutPrefix(path, home+string(filepath.Separator)); found {
		return "~" + string(filepath.Separator) + rest
	}
	return path
}

// FoldLines applies Fold to each newline-separated path in paths.
func FoldLines(paths string) string {
	lines := strings.Split(paths, "\n")
	for i, line := range lines {
		lines[i] = Fold(line)
	}
	return strings.Join(lines, "\n")
}

// FoldText applies Fold to every absolute path embedded in text, such as an
// error message or native diagnostic report. An occurrence of the home
// directory is folded only where it starts a path, rather than continuing a
// longer path or name, and ends at a segment boundary.
func FoldText(text string) string {
	home := homeDir()
	if home == "" {
		return text
	}
	var b strings.Builder
	for {
		at := strings.Index(text, home)
		if at < 0 {
			b.WriteString(text)
			return b.String()
		}
		end := at + len(home)
		before, _ := utf8.DecodeLastRuneInString(text[:at])
		after, _ := utf8.DecodeRuneInString(text[end:])
		b.WriteString(text[:at])
		if (at == 0 || !continuesPath(before)) && (end == len(text) || after == filepath.Separator || !continuesPath(after)) {
			b.WriteString("~")
		} else {
			b.WriteString(home)
		}
		text = text[end:]
	}
}

// continuesPath reports whether r, adjacent to a candidate home directory,
// would make it part of a longer path or file name.
func continuesPath(r rune) bool {
	return r == filepath.Separator || r == '.' || r == '-' || r == '_' || r == '~' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// homeDir returns the cleaned absolute home directory, or "" when it is
// unknown or is the filesystem root, where folding would rewrite every path.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return ""
	}
	home = filepath.Clean(home)
	if home == string(filepath.Separator) {
		return ""
	}
	return home
}
