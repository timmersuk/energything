package backup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// Mountinfo catches bind mounts and volumes covering the runtime. The root
// container overlay is allowed; a separate runtime/tmpfs mount is not.
func validateRuntimeMount(path string) error {
	data, e := os.ReadFile("/proc/self/mountinfo")
	if e != nil {
		return e
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		mount := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\134`, `\`).Replace(fields[4])
		if mount == "/" {
			continue
		}
		rel, e := filepath.Rel(mount, path)
		if e == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))) {
			return errors.New("runtime database must be in the ephemeral container layer, never a mounted directory")
		}
	}
	return nil
}
