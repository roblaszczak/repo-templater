package templater

import (
	"bytes"
	"os"
	"os/exec"
)

func (t Templater) runGitCmd(runDirectory string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = runDirectory

	t.Logger.Printf("running: git %s in %s", args, runDirectory)

	return cmd.Run()
}

func (t Templater) gitAdd(runDirectory, toAdd string) error {
	return t.runGitCmd(runDirectory, "add", toAdd)
}

func (t Templater) showGitDiff(runDirectory string) error {
	return t.runGitCmd(runDirectory, "--no-pager", "diff", "--cached")
}

func (t Templater) gitCommit(runDirectory, msg string) error {
	return t.runGitCmd(runDirectory, "commit", "-m", msg)
}

func (t Templater) gitPush(runDirectory string) error {
	return t.runGitCmd(runDirectory, "push")
}

func (t Templater) hasUncommittedChanges(runDirectory string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Stderr = os.Stderr
	statusBuf := bytes.NewBuffer(nil)
	cmd.Stdout = statusBuf
	cmd.Dir = runDirectory

	if err := cmd.Run(); err != nil {
		return false, err
	}

	return len(statusBuf.String()) > 0, nil
}
