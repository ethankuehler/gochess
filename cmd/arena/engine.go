package main

import (
	"io"

	"github.com/ethankuehler/gochess/uci"
)

type engineOption struct {
	name  string
	value string
}

type engineSearchResult = uci.EngineResult

func startEngine(path string, options []engineOption, errOut io.Writer) (*uci.ProcessClient, error) {
	base := []engineOption{
		{name: "Threads", value: "1"},
		{name: "Hash", value: "16"},
		{name: "Ponder", value: "false"},
		{name: "Move Overhead", value: "0"},
	}
	all := append(base, options...)
	processOptions := make([]uci.ProcessOption, len(all))
	for i, option := range all {
		processOptions[i] = uci.ProcessOption{Name: option.name, Value: option.value}
	}
	return uci.StartProcess(path, processOptions, errOut)
}
