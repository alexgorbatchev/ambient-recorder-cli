package main

import (
	"errors"
	"os"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
)

func withServiceVersion(version string, run func() error) (err error) {
	path := os.Getenv(serviceinfo.Environment)
	if path == "" {
		return run()
	}
	s, err := serviceinfo.Listen(path, version)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	return run()
}
