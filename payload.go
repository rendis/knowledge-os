// Package knowledgeos carries the distribution's kernel payload inside the kos binary, so a cell can
// update its kernel from the binary alone.
package knowledgeos

import "embed"

// Payload holds the kernel, the adapters and the list of paths the distribution owns in a cell.
//
//go:embed all:kernel all:adapters MANAGED_PATHS
var Payload embed.FS
