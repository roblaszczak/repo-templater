package templater_test

import (
	"fmt"
	"github.com/roblaszczak/repo-templater/pkg/templater"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"
)

const examplesDir = "../../examples"

func TestExamples(t *testing.T) {
	examples := examplesList(t)

	fmt.Println(examples)

	for i := range examples {
		example := examples[i]

		if example == "real-example" {
			continue
		}

		exampleDir := path.Join(examplesDir, example)

		configDirectory := path.Join(exampleDir, "config")
		inputDirectory := path.Join(exampleDir, "input")
		expectedOutputDirectory := path.Join(exampleDir, "output")
		testOutputDir := path.Join(exampleDir, "test_output")

		removeContents(t, testOutputDir)

		t.Run(examples[i], func(t *testing.T) {
			//t.Parallel()
			// todo - reenable

			tplr := templater.Templater{
				InputDirectory:  inputDirectory,
				OutputDirectory: testOutputDir,
				ConfigDirectory: configDirectory,
				Logger:          log.New(os.Stderr, "[example] ", log.LstdFlags),
			}

			config, err := tplr.ParseConfig(configDirectory)
			require.NoError(t, err)

			for _, repo := range config.Repositories {
				err := filepath.Walk(path.Join(inputDirectory, repo.Name), func(path string, info os.FileInfo, err error) error {
					if info.IsDir() {
						return nil
					}

					destPath := filepath.Join(testOutputDir, path[len(inputDirectory):])

					if err := tplr.CopyFile(path, destPath); err != nil {
						return err
					}

					return nil
				})
				require.NoError(t, err)
			}

			require.NoError(t, tplr.Run(*config))

			assertDirectoriesEquals(t, expectedOutputDirectory, testOutputDir)
		})
	}
}

func examplesList(t *testing.T) []string {
	var dirs []string

	files, err := ioutil.ReadDir(examplesDir)
	require.NoError(t, err)

	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, file.Name())
		}
	}

	return dirs
}

func removeContents(t *testing.T, dir string) {
	d, err := os.Open(dir)
	if os.IsNotExist(err) {
		return
	}

	require.NoError(t, err)
	defer func() {
		require.NoError(t, d.Close())
	}()

	names, err := d.Readdirnames(-1)
	require.NoError(t, err)

	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		require.NoError(t, err)
	}
}

func assertDirectoriesEquals(t *testing.T, expectedDirectory, actualDirectory string) {
	err := filepath.Walk(expectedDirectory, func(expectedFile string, info os.FileInfo, err error) error {
		actualFile := filepath.Join(actualDirectory, expectedFile[len(expectedDirectory):])

		if info.IsDir() {
			if !assert.DirExists(t, actualFile) {
				return nil
			}
		} else {
			if !assert.FileExists(t, actualFile) {
				return nil
			}
		}

		if info.IsDir() {
			return nil
		}

		actualFileContent, err := ioutil.ReadFile(actualFile)
		require.NoError(t, err)

		expectedFileContent, err := ioutil.ReadFile(expectedFile)
		require.NoError(t, err)

		assert.Equal(t, string(expectedFileContent), string(actualFileContent))

		return nil
	})
	assert.NoError(t, err)

	// todo - check for extra files
}
