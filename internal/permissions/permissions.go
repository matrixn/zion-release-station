package permissions

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type Report struct {
	Status     string `json:"status"`
	Readable   bool   `json:"readable"`
	Writable   bool   `json:"writable"`
	Deployable bool   `json:"deployable"`
	Message    string `json:"message"`
}

func Check(path string) (Report, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Report{Status: "missing", Message: "The directory does not exist."}, nil
		}
		return Report{}, fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return Report{Status: "invalid", Message: "The path is not a directory."}, nil
	}

	readable := unix.Access(path, unix.R_OK) == nil
	writable := unix.Access(path, unix.W_OK) == nil
	report := Report{Readable: readable, Writable: writable, Deployable: readable && writable}
	switch {
	case !readable:
		report.Status = "unavailable"
		report.Message = "ReleaseStation cannot read this directory."
	case !writable:
		report.Status = "read_only"
		report.Message = "The directory is readable but not writable by the ReleaseStation package user."
	default:
		report.Status = "ready"
		report.Message = "The directory is readable and writable by the ReleaseStation package user."
	}
	return report, nil
}
