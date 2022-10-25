package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ShowRefDescription gets the description for the specified commit ref. If it succeeds, s contains
// the description and err is nil. If it fails, s contains the error output and err contains the
// error returned from Run().
func FormatShowRefDescription(ref, format string) (s string, err error) {
	if output, err := gitOutput("show", ref, "--no-patch", "--no-color", fmt.Sprintf("--format=%s", format)); err != nil {
		return "", err
	} else {
		return strings.TrimSpace(output), nil
	}
}

// Diff shows the diff/patch between two specific commits. If it succeeds, buf contains the patch
// and err is nil. If it fails, buf contains the error output and err contains the error returned
// from Run()
func Diff(ref1, ref2 string) (buf *bytes.Buffer, err error) {
	buf = &bytes.Buffer{}
	cmd := exec.Command("git", "diff", ref1, ref2, "-p", "--no-color")
	cmd.Stdout = buf
	cmd.Stderr = buf

	err = cmd.Run()
	return
}

func IsDifferent(ref1, ref2 string) (bool, error) {
	buf, err := Diff(ref1, ref2)
	if err != nil {
		return true, err
	} else if buf.Len() == 0 {
		return false, nil
	}
	return true, nil
}

// ApplyPatch applies the patch in buf to the working tree but doesn't add or commit it.
func ApplyPatch(r io.Reader) error {
	// we use --recount instead of trying to manually fix patch chunks ourselves
	cmd := exec.Command("git", "apply", "--recount", "-")
	cmd.Stdin = r

	if output, err := cmd.CombinedOutput(); err != nil {
		asExecuted := cmd.String()
		return fmt.Errorf("%s: %s\n%s", err, asExecuted, output)
	}
	return nil
}

// HasChanges returns true if there are changes that have not been committed in the working tree
func HasChanges() (bool, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "status", "-s")
	cmd.Stdout = buf

	err := cmd.Run()
	if err != nil {
		return true, fmt.Errorf("error running `git status -s`: %s", err)
	}

	var line string
	for reader := bufio.NewReader(buf); err == nil; line, err = reader.ReadString('\n') {
		if len(line) == 0 {
			continue
		}
		line = strings.TrimSpace(line)
		switch line[0] {
		case '?':
			continue
		default:
			return true, nil
		}
	}
	return false, nil
}

// GetCurrentBranchName gets the current branch name
func GetCurrentBranchName() (name string, err error) {
	if output, err := exec.Command("git", "branch", "--show-current").CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: %s", err, output)
	} else {
		return strings.TrimSpace(string(output)), nil
	}
}

// BranchExists returns whether or not the specified branch name exists
func BranchExists(name string) bool {
	_, err := RevParse(name)
	return err == nil
}

// Commit triggers a commit, bringing up the default editor with the specified message
func Commit(message string) error {
	cmd := exec.Command("git", "commit", "-F", "-")
	cmd.Stdin = strings.NewReader(message)

	if output, err := cmd.CombinedOutput(); err != nil {
		asExecuted := cmd.String()
		return fmt.Errorf("%s: %s\n%s", err, asExecuted, output)
	}
	return nil
}

// Amend runs `git commit --amend` to amend the details of the last commit. It binds to the terminal
// so that in-terminal editors like vim can be used "normally"
func Amend() error {
	cmd := exec.Command("git", "commit", "--amend")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AmendWithMessage runs `git commit --amend -m <message>`
func AmendWithMessage(message string) error {
	return git("commit", "--amend", "-m", message)
}

// Amend runs `git commit --amend --no-edit` to amend the details of the last commit
func AmendNoEdit() error {
	return git("commit", "--amend", "--no-edit")
}

// Checkout the specified ref
func Checkout(ref string) error {
	return git("checkout", ref)
}

// CreateAndSwitchToBranch creates a new branch and switches to it (`git checkout -b`)
func CreateAndSwitchToBranch(branchName string) error {
	return git("checkout", "-b", branchName)
}

// CreateBranch creates a branch at HEAD but doesn't switch to it
func CreateBranch(branchName string) error {
	return git("branch", branchName)
}

// ForceDeleteBranch force-deletes the specified branch
func ForceDeleteBranch(branchName string) error {
	return git("branch", "-D", branchName)
}

// RevParse gets the hash for a ref
func RevParse(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	if output, err := cmd.CombinedOutput(); err != nil {
		asExecuted := cmd.String()
		return "", fmt.Errorf("%s: %s\n%s", err, asExecuted, output)
	} else {
		return strings.TrimSpace(string(output)), nil
	}
}

// Add does a `git add`
func Add(paths ...string) error {
	arg := append([]string{"add", "--"}, paths...)
	return git(arg...)
}

// Rebase does a `git rebase`
func Rebase(base, topic string) error {
	return git("rebase", base, topic)
}

// Log returns a log as per the provided arguments
func Log(arg ...string) (string, error) {
	arg = append([]string{"log"}, arg...)
	return gitOutput(arg...)
}

// GetForkPoint returns the common ancestor commit of the specified refs
func GetForkPoint(ref string, arg ...string) (string, error) {
	arg = append([]string{"merge-base", "--fork-point", ref}, arg...)
	if output, err := gitOutput(arg...); err != nil {
		// verified that an error is returned when fully merged or no common ancestor exists
		return output, err
	} else {
		return output, nil
	}
}

func GetPushRemoteForBranch(branch string) (string, error) {
	pushRemotePath := fmt.Sprintf("branch.%s.pushRemote", branch)
	remotePath := fmt.Sprintf("branch.%s.remote", branch)

	if pushRemote, err := gitOutput("config", "--get", pushRemotePath); err == nil {
		// if pushRemote is specified, use it
		return pushRemote, nil
	} else if remote, err := gitOutput("config", "--get", remotePath); err != nil {
		// otherwise try to use remote
		return "", err
	} else {
		return remote, nil
	}
}

// Push does a `git push`
func Push() error {
	return git("push")
}

// PushBranch pushes a branch to its default remote without switching to it.
func PushBranch(branch string) error {
	if remote, err := GetPushRemoteForBranch(branch); err != nil {
		return err
	} else {
		return git("push", remote, branch)
	}
}

// ForcePushBranch pushes a branch to its default remote without switching to it.
func ForcePushBranch(branch string) error {
	if remote, err := GetPushRemoteForBranch(branch); err != nil {
		return err
	} else {
		return git("push", "-f", remote, branch)
	}
}

// PushAndSetUpstream sets the remote tracking branch and pushes
func PushAndSetUpstream(remote, branch string) error {
	return git("push", "-u", remote, branch)
}

func gitOutput(arg ...string) (string, error) {
	cmd := exec.Command("git", arg...)

	if output, err := cmd.CombinedOutput(); err != nil {
		asExecuted := cmd.String()
		return "", fmt.Errorf("%s: %s\n%s", err, asExecuted, output)
	} else {
		return strings.TrimSpace(string(output)), nil
	}
}

func git(arg ...string) error {
	_, err := gitOutput(arg...)
	return err
}
