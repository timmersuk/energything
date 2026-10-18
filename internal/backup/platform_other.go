//go:build !linux

package backup

// Directory fsync is unavailable through os.File on Windows. Atomic replacement
// semantics are verified in Linux container tests.
func syncDirectory(string) error         { return nil }
func checkPrivateDirectory(string) error { return nil }
