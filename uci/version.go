// Package uci implements the Universal Chess Interface for the gochess engine:
// the command loop, position and go parsing, clock allocation, info formatting,
// options, and a deterministic benchmark.
package uci

// EngineName is the protocol-visible engine name.
const EngineName = "gochess"

// Author is the protocol-visible author.
const Author = "Ethan Kuehler"

// Version is the engine version. It can be overridden at build time with
// -ldflags "-X github.com/ethankuehler/gochess/uci.Version=..."
var Version = "0.1.0"

// VersionString returns the human-readable engine name and version.
func VersionString() string {
	return EngineName + " " + Version
}
