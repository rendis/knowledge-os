package main

import (
	"os"
	"testing"
)

// Command tests run uncached.
func TestMain(m *testing.M) {
	os.Setenv("KOS_NO_CACHE", "1")
	os.Exit(m.Run())
}
