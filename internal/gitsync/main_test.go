package gitsync

import (
	"os"
	"testing"
)

// Sync tests change notes and repositories between verifications in one process; they run uncached.
func TestMain(m *testing.M) {
	os.Setenv("KOS_NO_CACHE", "1")
	os.Exit(m.Run())
}
