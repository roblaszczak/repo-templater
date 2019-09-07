package main

import (
	"flag"
	"github.com/roblaszczak/repo-templater/pkg/templater"
	"log"
	"os"
	"strings"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	// todo
	var runCommands arrayFlags
	var skipRepos arrayFlags

	commitMsg := flag.String("commit-msg", "update repository template", "")
	push := flag.Bool("push", false, "")
	doNotRemove := flag.Bool("do-not-remove", false, "")
	repository := flag.String("repository", "", "limit run to one repository")
	flag.Var(&skipRepos, "skip-repository", "limit run to one repository")
	branch := flag.String("branch", "clone repositories from provided branch", "")
	flag.Var(&runCommands, "run-command", "commands to run, can be set multiple times to run multiple commands")

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

	if len(runCommands) > 0 {
		for _, runCommand := range runCommands {
			for _, repository := range config.Repositories {
				repository.RunCmds = append(repository.RunCmds, strings.Split(runCommand, " "))
			}
		}
	}

	defer func() {
		if *doNotRemove {
			return
		}
		if err := os.RemoveAll("input"); err != nil {
			logger.Printf("cannot remove dir: %s", err)
		}
	}()

	if err := t.ReallyRun(*config, *branch, *repository, skipRepos); err != nil {
		panic(err)
	}
}
