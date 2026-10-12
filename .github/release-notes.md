## Summary

- `config print-file` prints the absolute path of the selected configuration file, using `--config` or the XDG default. A missing file leaves stdout empty and exits 1. The command checks existence without parsing TOML or creating files.
- The recording-directory command moves from `config print-dir` to `recording print-dir`. Update scripts that use the old command; `config print-dir` is removed and returns an error.
- All path commands print only an absolute path and a newline in human and agent modes. `recording print-file` continues to report the currently open recording file.

Executables are ad-hoc signed; they are not Developer ID signed or notarized. Transcription, VAD, and calendar integration are not included.
