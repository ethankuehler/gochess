package main

import (
	"io"

	"github.com/ethankuehler/gochess/uci"
)

type stockfishClient = uci.ProcessClient

func startStockfish(path string, errOut io.Writer) (*stockfishClient, error) {
	return uci.StartProcess(path, []uci.ProcessOption{
		{Name: "Threads", Value: "1"},
		{Name: "Hash", Value: "16"},
	}, errOut)
}
