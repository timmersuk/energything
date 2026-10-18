//go:build !linux

package backup

// Directory fsync is unavailable through os.File on Windows. Atomic replacement
// semantics and the production mount gate are verified in Linux container tests.
func syncDirectory(string) error        { return nil }
func validateRuntimeMount(string) error { return nil }
