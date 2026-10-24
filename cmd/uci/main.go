// Command uci is the UCI front end for the gochess engine. It embeds the
// attack tables, so it can be copied to another directory and launched by a
// chess GUI without a data directory.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ethankuehler/gochess/uci"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bench":
			depth := 5
			if len(os.Args) > 2 {
				if value, err := strconv.Atoi(os.Args[2]); err == nil && value > 0 {
					depth = value
				}
			}
			os.Exit(uci.Bench(os.Stdout, os.Stderr, depth))
		case "-version", "--version", "version":
			fmt.Println(uci.VersionString())
			return
		case "-h", "--help", "help":
			fmt.Println("usage: gochess [bench [depth] | -version]")
			fmt.Println("With no arguments, gochess speaks UCI on stdin/stdout.")
			return
		default:
			fmt.Fprintf(os.Stderr, "gochess: ignoring unknown argument %q\n", os.Args[1])
		}
	}

	os.Exit(uci.Run(os.Stdin, os.Stdout, os.Stderr))
}
