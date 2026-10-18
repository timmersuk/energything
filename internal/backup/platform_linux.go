package backup

import (
	"errors"
	"os"
	"syscall"
)

func checkPrivateDirectory(path string) error {
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0 {
		return errors.New("storage directory must be private (0700) and owned by the service user (container UID 10001)")
	}
	return nil
}

func syncDirectory(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	return errors.Join(f.Sync(), f.Close())
}
