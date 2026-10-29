package homepath

import "testing"

func TestFold(t *testing.T) {
	tests := []struct {
		name, home, path, want string
	}{
		{"home itself", "/home/dev", "/home/dev", "~"},
		{"nested path", "/home/dev", "/home/dev/projects/app", "~/projects/app"},
		{"sibling sharing the prefix", "/home/dev", "/home/developer/app", "/home/developer/app"},
		{"outside home", "/home/dev", "/var/recordings", "/var/recordings"},
		{"relative path", "/home/dev", "home/dev/app", "home/dev/app"},
		{"home with trailing separator", "/home/dev/", "/home/dev/app", "~/app"},
		{"unknown home", "", "/home/dev/app", "/home/dev/app"},
		{"relative home", "home/dev", "home/dev/app", "home/dev/app"},
		{"root home", "/", "/var/recordings", "/var/recordings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.home)
			if got := Fold(tt.path); got != tt.want {
				t.Fatalf("Fold(%q) with HOME=%q = %q, want %q", tt.path, tt.home, got, tt.want)
			}
		})
	}
}

func TestFoldLines(t *testing.T) {
	t.Setenv("HOME", "/home/dev")
	got := FoldLines("/home/dev/a.opus\n/home/developer/b.opus\n/home/dev")
	if want := "~/a.opus\n/home/developer/b.opus\n~"; got != want {
		t.Fatalf("FoldLines = %q, want %q", got, want)
	}
}

func TestFoldText(t *testing.T) {
	tests := []struct {
		name, home, text, want string
	}{
		{"quoted nested path", "/home/dev", `inspect configuration "/home/dev/.config/a.toml": no such file`, `inspect configuration "~/.config/a.toml": no such file`},
		{"home itself at end", "/home/dev", "service home is /home/dev", "service home is ~"},
		{"home itself before punctuation", "/home/dev", `cd "/home/dev": denied`, `cd "~": denied`},
		{"several paths", "/home/dev", "open /home/dev/a: copy to /home/dev/b", "open ~/a: copy to ~/b"},
		{"sibling sharing the prefix", "/home/dev", "open /home/developer/a", "open /home/developer/a"},
		{"sibling file name", "/home/dev", "open /home/dev.bak/a", "open /home/dev.bak/a"},
		{"home nested in another path", "/home/dev", "open /mnt/home/dev/a", "open /mnt/home/dev/a"},
		{"outside home", "/home/dev", "open /var/a", "open /var/a"},
		{"key-value field", "/home/dev", `plist="/home/dev/Library/a.plist"`, `plist="~/Library/a.plist"`},
		{"unknown home", "", "open /home/dev/a", "open /home/dev/a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.home)
			if got := FoldText(tt.text); got != tt.want {
				t.Fatalf("FoldText(%q) with HOME=%q = %q, want %q", tt.text, tt.home, got, tt.want)
			}
		})
	}
}
