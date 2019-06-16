package main

import (
	"github.com/roblaszczak/repo-templater/pkg/templater"
	"log"
	"os"
)

func main() {
	t := templater.Templater{
		InputDirectory:  "input",
		OutputDirectory: "input",
		ConfigDirectory: ".",
		Logger:          log.New(os.Stderr, "[example] ", log.LstdFlags),
	}
	config, err := t.ParseConfig(".")
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat("input"); err == nil {
		if err := os.RemoveAll("input"); err != nil {
			panic(err)
		}
	}

	if err := os.Mkdir("input", 0755); err != nil {
		panic(err)
	}

	if err := t.ReallyRun(config); err != nil {
		panic(err)
	}

	if err := os.RemoveAll("input"); err != nil {
		panic(err)
	}
}
