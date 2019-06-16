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
	"text/template"
)

const ConfigFile = ".repo-templater.toml"
const TemplatesDirectory = ".repo-templates"

type Templater struct {
	InputDirectory  string
	OutputDirectory string
	ConfigDirectory string
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

	for _, repository := range config.Repositories {
		cmd := exec.Command("git", "add", ".")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = path.Join(t.InputDirectory, repository.Name)

		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "cannot clone %s", repository.Name)
		}

		cmd = exec.Command("git", "--no-pager", "diff", "--cached")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = path.Join(t.InputDirectory, repository.Name)

		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "cannot clone %s", repository.Name)
		}

		cmd = exec.Command("git", "status", "--porcelain")
		cmd.Stderr = os.Stderr
		statusBuf := bytes.NewBuffer(nil)
		cmd.Stdout = statusBuf
		cmd.Dir = path.Join(t.InputDirectory, repository.Name)

		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "cannot clone %s", repository.Name)
		}

		if len(statusBuf.String()) == 0 {
			t.Logger.Printf("no changes detected")
			continue
		}

		if !prompt() {
			continue
		}

		cmd = exec.Command("git", "commit", "-m", "update repository template")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = path.Join(t.InputDirectory, repository.Name)
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "cannot clone %s", repository.Name)
		}

		cmd = exec.Command("git", "push")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = path.Join(t.InputDirectory, repository.Name)

		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "cannot clone %s", repository.Name)
		}
	}

	return nil
}

func (t Templater) ParseConfig(configDir string) (Config, error) {
	config := Config{}
	if _, err := toml.DecodeFile(path.Join(configDir, ConfigFile), &config); err != nil {
		return config, err
	}

	return config, nil
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

				if err := t.renderFile(path, destPath, repository); err != nil {
					return err
				}

				return nil
			})
		}

		if err != nil {
			return errors.Wrap(err, "cannot read input directory")
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

	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, config); err != nil {
		// todo - why it is not propagating when occurred?
		return err
	}

	err = ioutil.WriteFile(destFile, buf.Bytes(), sourceStat.Mode())
	if err != nil {
		return err
	}

	return nil
}

func prompt() bool {
	for {
		fmt.Print("y/n ?")
		var input string
		fmt.Scanln(&input)

		if input == "" {
			continue
		}

		return input == "y"
	}
}
