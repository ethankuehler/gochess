// Package uci implements the Universal Chess Interface.
package uci

const EngineName = "gochess"
const Author = "Ethan Kuehler"

// Version can be overridden with -ldflags -X.
var Version = "0.5.0"

func VersionString() string {
	return EngineName + " " + Version
}
