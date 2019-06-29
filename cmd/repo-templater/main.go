package main

import (
	"flag"
	"github.com/roblaszczak/repo-templater/pkg/templater"
	"log"
	"os"
)

func main() {
	commitMsg := flag.String("commit-msg", "update repository template", "")
	push := flag.Bool("push", false, "")
	doNotRemove := flag.Bool("do-not-remove", false, "")
	repository := flag.String("repository", "limit run to one repository", "")     // todo
	branch := flag.String("branch", "clone repositories from provided branch", "") // todo

	flag.Parse()

	logger := log.New(os.Stderr, "[templater] ", log.LstdFlags)

	t := templater.Templater{
		InputDirectory:  "input",
		OutputDirectory: "input",
		ConfigDirectory: ".",
		CommitMessage:   *commitMsg,
		Push:            *push,
		Logger:          logger,
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

	defer func() {
		if *doNotRemove {
			return
		}
		if err := os.RemoveAll("input"); err != nil {
			logger.Printf("cannot remove dir: %s", err)
		}
	}()

	if err := t.ReallyRun(*config, *branch, *repository); err != nil {
		panic(err)
	}
}
