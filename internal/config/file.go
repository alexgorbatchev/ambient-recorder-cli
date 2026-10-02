package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Load overlays a TOML file on defaults. An absent implicit default is optional;
// an explicitly selected file must exist. Validate after applying CLI overrides.
func Load(path string) (Settings, string, error) {
	cfg, err := Defaults()
	if err != nil {
		return Settings{}, "", err
	}
	explicit := path != ""
	path, err = resolvePath(path)
	if err != nil {
		return Settings{}, "", err
	}
	f, err := os.Open(path)
	if !explicit && errors.Is(err, os.ErrNotExist) {
		return cfg, "", nil
	}
	if err != nil {
		return Settings{}, "", fmt.Errorf("open configuration %s: %w", path, err)
	}
	err = errors.Join(toml.NewDecoder(f).DisallowUnknownFields().Decode(&cfg), f.Close())
	if err != nil {
		return Settings{}, "", fmt.Errorf("read configuration %s: %w", path, err)
	}
	return cfg, path, nil
}

// Create writes defaults without replacing an existing configuration.
func Create(path string) error {
	path, err := resolvePath(path)
	if err != nil {
		return err
	}
	cfg, err := Defaults()
	if err != nil {
		return err
	}
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode default configuration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create configuration %s: %w", path, err)
	}
	_, writeErr := f.Write(b)
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return fmt.Errorf("write configuration %s: %w", path, err)
	}
	return nil
}

func resolvePath(path string) (string, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return "", err
		}
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve configuration path: %w", err)
	}
	return path, nil
}
