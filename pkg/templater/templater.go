package templater

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

const ConfigFile = ".repo-templater.toml"
const TemplatesDirectory = ".repo-templates"

type Templater struct {
	InputDirectory  string
	OutputDirectory string
	ConfigDirectory string
	CommitMessage   string
	Push            bool
	Logger          *log.Logger
}

func (t Templater) repositoriesToRun(config Config, repositoryFilter string) []*RepositoryConfig {
	if repositoryFilter == "" {
		return config.Repositories
	}

	for _, repo := range config.Repositories {
		if repo.Name == repositoryFilter {
			return []*RepositoryConfig{repo}
		}
	}

	panic("repository not found")
}

func (t Templater) ReallyRun(config Config, branch string, repoFilter string) error {
	reposToRun := t.repositoriesToRun(config, repoFilter)

	cloneWg := sync.WaitGroup{}
	cloneWg.Add(len(reposToRun))

	if branch == "" {
		branch = "master"
	}

	for i := range reposToRun {
		go func(repository *RepositoryConfig) {
			cmd := exec.Command(
				"git",
				"clone",
				repository.URL,
				repository.Name,
				"--single-branch",
				"--branch",
				branch,
			)

			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			cmd.Dir = t.InputDirectory

			if err := cmd.Run(); err != nil {
				panic(errors.Wrapf(err, "cannot clone %s", repository.Name))
			}

			cloneWg.Done()
		}(reposToRun[i])
	}

	cloneWg.Wait()

	if err := t.Run(config, reposToRun); err != nil {
		return err
	}

	var repositoriesWithChanges []*RepositoryConfig

	for _, repository := range reposToRun {
		repositoryDir := path.Join(t.InputDirectory, repository.Name)

		if err := t.gitAdd(repositoryDir, "."); err != nil {
			return errors.Wrapf(err, "git add failed for %s", repository.Name)
		}

		fmt.Printf("\nDiff for repository %s\n", repository.Name)
		if err := t.showGitDiff(repositoryDir); err != nil {
			return errors.Wrapf(err, "cannot show diff %s", repository.Name)
		}

		needCommit, err := t.hasUncommittedChanges(repositoryDir)
		if err != nil {
			return errors.Wrapf(err, "check for changes in %s", repository.Name)
		}

		if !needCommit {
			t.Logger.Printf("%s doesn't need update", repository.Name)
			continue
		}

		if !t.Push {
			continue
		}

		if !prompt("do you want to commit these changes?") {
			continue
		}

		if err := t.gitCommit(repositoryDir, t.CommitMessage); err != nil {
			return errors.Wrapf(err, "cannot commit changes in %s", repository.Name)
		}

		repositoriesWithChanges = append(repositoriesWithChanges, repository)
	}

	if t.Push {
		for _, repository := range repositoriesWithChanges {
			if err := t.gitPush(path.Join(t.InputDirectory, repository.Name)); err != nil {
				return errors.Wrapf(err, "cannot push %s", repository.Name)
			}
		}
	} else {
		t.Logger.Println("dry run, not pushing changes, to push please add -push flag")
	}

	return nil
}

func (t Templater) ParseConfig(configDir string) (*Config, error) {
	config := &Config{}
	if _, err := toml.DecodeFile(path.Join(configDir, ConfigFile), &config); err != nil {
		return nil, err
	}

	for repositoryNum := range config.Repositories {
		for key, value := range config.CommonVariables {
			repoConfig := config.Repositories[repositoryNum]

			if repoConfig.Variables == nil {
				repoConfig.Variables = map[string]string{}
			}
			if _, ok := repoConfig.Variables[key]; !ok {
				repoConfig.Variables[key] = value
			}
		}
	}

	for repositoryNum := range config.Repositories {
		repoConfig := config.Repositories[repositoryNum]

		varsToTemplate := []*string{
			&repoConfig.Name,
			&repoConfig.HumanName,
			&repoConfig.URL,
		}

		variablesToTemplateMap := map[string]*string{}

		for key := range repoConfig.Variables {
			v := repoConfig.Variables[key]
			vPtr := &v
			variablesToTemplateMap[key] = vPtr
			varsToTemplate = append(varsToTemplate, vPtr)
		}

	TemplatingLoop:
		for {
			for key, value := range variablesToTemplateMap {
				repoConfig.Variables[key] = *value
			}

			toTemplateCount := len(varsToTemplate)
			templatedCount := 0
			for _, toTemplate := range varsToTemplate {
				if !isTemplated(*toTemplate) {
					toTemplateCount--
					continue
				}

				nameTemplated := templateVar(*toTemplate, repoConfig, config)
				if nameTemplated != "" {
					*toTemplate = nameTemplated
					toTemplateCount--
					templatedCount++
					fmt.Println("templated ", nameTemplated)
					continue TemplatingLoop
				}
			}

			if toTemplateCount == 0 {
				break TemplatingLoop
			}
			if templatedCount == 0 {
				panic(fmt.Sprintf("cannot template more, missing templates: %d\n", toTemplateCount))
			}
		}
	}

	return config, nil
}

func templateVar(variable string, repoConfig *RepositoryConfig, config *Config) string {
	tpl := template.Must(template.New("tpl").Parse(variable))
	tpl.Option("missingkey=error")

	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, makeTemplateVariables(*repoConfig, *config)); err != nil {
		fmt.Println("@@@@@@@@2 templating err,", err.Error())
		return ""
	}

	s := buf.String()
	if isTemplated(s) {
		fmt.Printf(">>>>>\nnot templated:\n%s\n>>>>>\n", s)
		return ""
	}

	return s
}

func isTemplated(s string) bool {
	return strings.Contains(s, "{{") && strings.Contains(s, "}}")
}

// todo - err wraps
func (t Templater) Run(config Config, repositoriesToRun []*RepositoryConfig) error {
	for _, repository := range repositoriesToRun {
		var err error

		for _, tpl := range repository.Templates {
			templateDir := path.Join(t.ConfigDirectory, TemplatesDirectory, tpl)

			err = filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					// todo - wtf?
					return err
				}

				if info.IsDir() {
					return nil
				}

				destPath := filepath.Join(t.OutputDirectory, repository.Name, path[len(templateDir):])
				t.Logger.Printf("copying file %s to %s", path, destPath)

				if err := t.renderFile(path, destPath, *repository, config); err != nil {
					return err
				}

				return nil
			})
		}

		if err != nil {
			return errors.Wrap(err, "cannot read input directory")
		}
	}

	cmdsWg := sync.WaitGroup{}
	cmdsWg.Add(len(repositoriesToRun))

	for i := range repositoriesToRun {
		go func(repository *RepositoryConfig) {
			for _, cmd := range repository.RunCmds {
				if err := t.runCmd(path.Join(t.OutputDirectory, repository.Name), cmd...); err != nil {
					panic(err)
				}
			}
			cmdsWg.Done()
		}(repositoriesToRun[i])
	}

	cmdsWg.Wait()

	return nil
}

// todo - make priv
func (t Templater) CopyFile(sourceFile, destFile string) error {
	destPath := filepath.Dir(destFile)
	sourcePath := filepath.Dir(sourceFile)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		sourceStat, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(destPath, sourceStat.Mode()); err != nil {
			return err
		}
	}

	sourceStat, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}

	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(destFile, input, sourceStat.Mode())
	if err != nil {
		return err
	}

	return nil
}

func (t Templater) renderFile(sourceFile, destFile string, repoConfig RepositoryConfig, config Config) error {
	destPath := filepath.Dir(destFile)
	sourcePath := filepath.Dir(sourceFile)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		sourceStat, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(destPath, sourceStat.Mode()); err != nil {
			return err
		}
	}

	sourceStat, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}

	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return err
	}

	t.Logger.Printf("rendering %s", sourceFile)

	tpl := template.Must(template.New("tpl").Parse(string(input)))
	tpl.Option("missingkey=error")

	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, makeTemplateVariables(repoConfig, config)); err != nil {
		// todo - why it is not propagating when occurred?
		panic(err)
	}

	err = ioutil.WriteFile(destFile, buf.Bytes(), sourceStat.Mode())
	if err != nil {
		return err
	}

	return nil
}

func prompt(message string) bool {
	for {
		fmt.Printf("%s [y/n]:", message)

		var input string
		_, _ = fmt.Scanln(&input)

		if input == "" {
			fmt.Print("\n")
			continue
		}

		return input == "y"
	}
}
