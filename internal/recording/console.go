package recording

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

const humanTimeFormat = "2006-01-02 15:04:05.000 -0700"

var consoleFields = map[string]string{
	"microphone_name": "microphone", "microphone_transport": "connection",
	"previous_microphone_name": "previous_microphone", "output_directory": "output",
	"playback": "playback", "path": "file", "error": "error", "log_error": "log_error",
	"retry_in": "retry_in", "dropped_frames": "dropped_frames", "clock_shift": "clock_shift",
}

func newConsoleLogger(w io.Writer, agent bool) *slog.Logger {
	if agent {
		options := &slog.HandlerOptions{Level: slog.LevelDebug}
		return slog.New(slog.NewJSONHandler(w, options))
	}
	options := &tint.Options{
		Level: slog.LevelDebug, TimeFormat: humanTimeFormat,
		NoColor: true, ReplaceAttr: consoleAttr,
	}
	if f, ok := w.(*os.File); ok && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		options.NoColor = !term.IsTerminal(int(f.Fd()))
	}
	return slog.New(tint.NewTextHandler(w, options))
}

func consoleAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key != slog.TimeKey && a.Key != slog.LevelKey && a.Key != slog.MessageKey {
		key, ok := consoleFields[a.Key]
		if !ok {
			return slog.Attr{}
		}
		a.Key = key
		if key == "playback" && a.Value.Kind() == slog.KindString && a.Value.String() == "system" {
			a.Value = slog.StringValue("all applications")
		}
	}
	var text string
	switch a.Value.Kind() {
	case slog.KindString:
		text = a.Value.String()
		if text == "" {
			return slog.Attr{}
		}
	case slog.KindAny:
		if a.Value.Any() == nil {
			return slog.Attr{}
		}
		if err, ok := a.Value.Any().(error); ok {
			text = err.Error()
		}
	}
	// Tint permits ANSI in colored values and raw newlines in messages. Keep
	// device names and joined errors within their event without altering JSON.
	if strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) {
		a.Value = slog.StringValue(strconv.Quote(text))
	}
	return a
}
