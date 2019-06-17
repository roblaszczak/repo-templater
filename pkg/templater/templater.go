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

func (t Templater) ReallyRun(config Config) error {
	for _, repository := range config.Repositories {
		cmd := exec.Command("git", "clone", repository.URL, repository.Name)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = t.InputDirectory

		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "cannot clone %s", repository.Name)
		}
	}

	if err := t.Run(config); err != nil {
		return err
	}

	var repositoriesWithChanges []*RepositoryConfig

	for _, repository := range config.Repositories {
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

func (t Templater) ParseConfig(configDir string) (Config, error) {
	config := Config{}
	if _, err := toml.DecodeFile(path.Join(configDir, ConfigFile), &config); err != nil {
		return config, err
	}

	for repositoryNum := range config.Repositories {
		for key, value := range config.CommonVariables {
			repoConfig := config.Repositories[repositoryNum]

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

				fmt.Println(makeTemplateVariables(*repoConfig))

				nameTemplated := templateVar(*toTemplate, repoConfig)
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
				// todo - err?
				fmt.Printf("cannot template more, missing templates: %d\n", toTemplateCount)
				break TemplatingLoop
			}
		}
	}

	return config, nil
}

func templateVar(variable string, config *RepositoryConfig) string {
	tpl := template.Must(template.New("tpl").Parse(variable))
	tpl.Option("missingkey=error")

	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, makeTemplateVariables(*config)); err != nil {
		return ""
	}

	s := buf.String()
	if isTemplated(s) {
		return ""
	}

	return s
}

func isTemplated(s string) bool {
	return strings.Contains(s, "{{") && strings.Contains(s, "}}")
}

// todo - err wraps
func (t Templater) Run(config Config) error {
	for _, repository := range config.Repositories {
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

				if err := t.renderFile(path, destPath, *repository); err != nil {
					return err
				}

				return nil
			})
		}

		if err != nil {
			return errors.Wrap(err, "cannot read input directory")
		}

		for _, cmd := range repository.RunCmds {
			if err := t.runCmd(path.Join(t.OutputDirectory, repository.Name), cmd...); err != nil {
				return errors.Wrap(err, "cannot run cmd")
			}
		}
	}

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

func (t Templater) renderFile(sourceFile, destFile string, config RepositoryConfig) error {
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
	if err := tpl.Execute(buf, makeTemplateVariables(config)); err != nil {
		// todo - why it is not propagating when occurred?
		return err
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
