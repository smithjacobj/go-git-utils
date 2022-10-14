package git

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// Test for Git utilities for git-split and gh-submit-stack. There are assumptions that `git` is on
// the PATH and is functioning properly. These tests only ensure that the utilities are behaving
// properly with the output provided by Git.

var k_FileNames = []string{"A", "B", "C", "D", "E", "F"}
var k_CommitDescriptions = make([]string, 0, len(k_FileNames))
var k_RefNames = make([]string, 0, len(k_FileNames))

func init() {
	for _, name := range k_FileNames {
		k_CommitDescriptions = append(k_CommitDescriptions, fmt.Sprintf("file %s", name))
	}
	for i := 0; i < len(k_FileNames); i++ {
		parentNum := len(k_FileNames) - i - 1
		k_RefNames = append(k_RefNames, fmt.Sprintf("HEAD~%d", parentNum))
	}
}

func expectEq[T comparable](t *testing.T, expected, actual T) {
	if expected != actual {
		t.Fatal("Expected", expected, "actual value", actual)
	}
}

func expectNEq[T comparable](t *testing.T, unexpected, actual T) {
	if unexpected == actual {
		t.Fatal("Expected", actual, "to not be", unexpected)
	}
}

func touch(name string) error {
	if f, err := os.Create(name); err != nil {
		return err
	} else if err = f.Close(); err != nil {
		return err
	}
	return nil
}

func commitBlankFile(name string) error {
	if err := touch(name); err != nil {
		return err
	} else if err := git("add", name); err != nil {
		return err
	} else if err := git("commit", "-m", fmt.Sprintf("file %s", name)); err != nil {
		return err
	}
	return nil
}

func setupGitRepo(t *testing.T) (cleanup func()) {
	folder, err := os.MkdirTemp(os.TempDir(), "go-git-utils-test")
	if err != nil {
		t.Fatal(err)
	}
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(folder); err != nil {
		t.Fatal(err)
	} else if err := git("init"); err != nil {
		t.Fatal(err)
	}

	for _, name := range k_FileNames {
		if err := commitBlankFile(name); err != nil {
			t.Fatal(err)
		}
	}

	return func() {
		os.Chdir(pwd)
		if err := os.RemoveAll(folder); err != nil {
			// not a test error, just messy
			t.Log(err)
		}
	}
}

func getConfigDefaultBranchName() (string, error) {
	return gitOutput("config", "--get", "init.defaultBranch")
}

func appendToFile(name, content string) error {
	f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return err
	}
	return nil
}

func TestFormatShowRefDescription(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	for i := 0; i < len(k_FileNames); i++ {
		ref := k_RefNames[i]
		expected := k_CommitDescriptions[i]
		if desc, err := FormatShowRefDescription(ref, "%B"); err != nil {
			t.Fatal(err)
		} else {
			expectEq(t, expected, desc)
		}
	}
}

func TestDiff(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	// we can do this because empty file hash is always e69de29
	expected := `diff --git a/B b/B
new file mode 100644
index 0000000..e69de29
diff --git a/C b/C
new file mode 100644
index 0000000..e69de29
diff --git a/D b/D
new file mode 100644
index 0000000..e69de29
diff --git a/E b/E
new file mode 100644
index 0000000..e69de29
diff --git a/F b/F
new file mode 100644
index 0000000..e69de29
`

	if actual, err := Diff(k_RefNames[0], k_RefNames[len(k_RefNames)-1]); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, expected, actual.String())
	}
}

func TestIsDifferent(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	if isDifferent, err := IsDifferent(k_RefNames[0], k_RefNames[1]); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, true, isDifferent)
	}

	if isDifferent, err := IsDifferent(k_RefNames[0], k_RefNames[0]); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, false, isDifferent)
	}
}

func TestApplyPatch(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	patch := `diff --git a/F b/F
--- a/F
+++ b/F
@@ -0,0 +0,5 @@
+1
+2
+3
+4
`

	expected := `1
2
3
4
`

	if err := ApplyPatch(strings.NewReader(patch)); err != nil {
		t.Fatal(err)
	}

	if f, err := os.Open("F"); err != nil {
		t.Fatal(err)
	} else if bs, err := io.ReadAll(f); err != nil {
		t.Fatal(err)
	} else {
		if err := f.Close(); err != nil {
			// another thing that's just messy if it doesn't work
			t.Log(err)
		}
		expectEq(t, expected, string(bs))
	}
}

func TestHasChanges(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	if hasChanges, err := HasChanges(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, false, hasChanges)
	}

	// apply a change and test again
	if err := appendToFile("F", "lorem ipsum"); err != nil {
		t.Fatal(err)
	} else if hasChanges, err := HasChanges(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, true, hasChanges)
	}
}

func TestGetCurrentBranchName(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	// test default branch name after init
	if configDefaultBranchName, err := getConfigDefaultBranchName(); err != nil {
		t.Fatal(err)
	} else if currentBranchName, err := GetCurrentBranchName(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, configDefaultBranchName, currentBranchName)
	}

	// test with new branch checkout
	if err := CreateAndSwitchToBranch("new-branch"); err != nil {
		t.Fatal(err)
	} else if currentBranchName, err := GetCurrentBranchName(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, "new-branch", currentBranchName)
	}
}

func TestBranchExists(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	// test an existing branch
	if err := CreateAndSwitchToBranch("exists"); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, true, BranchExists("exists"))
	}

	// test a nonexistent branch
	expectEq(t, false, BranchExists("does-not-exist"))
}

func TestCommit(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	expected := "file G"

	if err := touch("G"); err != nil {
		t.Fatal(err)
	} else if err := Add("G"); err != nil {
		t.Fatal(err)
	} else if err := Commit(expected); err != nil {
		t.Fatal(err)
	} else if desc, err := FormatShowRefDescription("HEAD", "%B"); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, expected, desc)
	}
}

func TestAmend(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	if err := appendToFile("F", "lorem ipsum"); err != nil {
		t.Fatal(err)
	} else if hasChanges, err := HasChanges(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, true, hasChanges)
	}

	if err := Add("."); err != nil {
		t.Fatal(err)
	} else if err := AmendNoEdit(); err != nil {
		// this has to be run with --no-edit or it shells out to the configured editor waiting for
		// user input, which breaks automated testing
		t.Fatal(err)
	} else if hasChanges, err := HasChanges(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, false, hasChanges)
	}

	if desc, err := FormatShowRefDescription("HEAD", "%B"); err != nil {
		t.Fatal(err)
	} else {
		// expect unchanged commit message
		expected := k_CommitDescriptions[len(k_CommitDescriptions)-1]
		expectEq(t, expected, desc)
	}
}

func TestCheckout(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	// k_RefNames aren't valid after this since they're relative to HEAD
	if err := Checkout(k_RefNames[3]); err != nil {
		t.Fatal(err)
	} else if desc, err := FormatShowRefDescription("HEAD", "%B"); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, k_CommitDescriptions[3], desc)
	}
}

func TestCreateAndSwitchToBranch(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	newBranchName := "a-new-branch"
	if output, err := gitOutput("branch", "-l"); err != nil {
		t.Fatal(err)
	} else if strings.Contains(output, newBranchName) {
		t.Fatal("Found our new branch " + newBranchName + " before we added it")
	}

	if err := CreateAndSwitchToBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if output, err := GetCurrentBranchName(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, newBranchName, output)
	}
}

func TestCreateBranch(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	newBranchName := "a-new-branch"
	if BranchExists(newBranchName) {
		t.Fatal("Found our new branch " + newBranchName + " before we added it")
	}

	if err := CreateBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if !BranchExists(newBranchName) {
		t.Fatal("Could not find branch named " + newBranchName)
	} else if output, err := GetCurrentBranchName(); err != nil {
		t.Fatal(err)
	} else if configDefaultBranchName, err := getConfigDefaultBranchName(); err != nil {
		t.Fatal(err)
	} else {
		expectEq(t, configDefaultBranchName, output)
		expectNEq(t, newBranchName, output)
	}
}

func TestForceDeleteBranch(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	newBranchName := "a-new-branch"
	// test delete without changes
	if err := CreateBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if !BranchExists(newBranchName) {
		t.Fatal("Could not find branch named " + newBranchName)
	} else if err := ForceDeleteBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if BranchExists(newBranchName) {
		t.Fatal("Found " + newBranchName + " after it should have been deleted")
	}

	// make some changes and commit them so there are changes not merged
	if err := CreateAndSwitchToBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if err := appendToFile("F", "lorem ipsum"); err != nil {
		t.Fatal(err)
	} else if hasChanges, err := HasChanges(); err != nil {
		t.Fatal(err)
	} else if !hasChanges {
		t.Fatal("Applied changes but no changes found")
	} else if err := Add("F"); err != nil {
		t.Fatal(err)
	} else if err := Commit("committing on " + newBranchName); err != nil {
		t.Fatal(err)
	} else if configDefaultBranchName, err := getConfigDefaultBranchName(); err != nil {
		t.Fatal(err)
	} else if err := Checkout(configDefaultBranchName); err != nil {
		t.Fatal(err)
	} else if err := ForceDeleteBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if BranchExists(newBranchName) {
		t.Fatal("Found " + newBranchName + " after it should have been deleted")
	}
}

func TestRevParse(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	// not a lot we can do here
	if _, err := RevParse("HEAD"); err != nil {
		t.Fatal(err)
	}
}

func TestAdd(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	if err := touch("Z"); err != nil {
		t.Fatal(err)
	} else if err := Add("Z"); err != nil {
		t.Fatal(err)
	} else if output, err := gitOutput("status", "-s"); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(output, "A  Z") {
		t.Fatal("Z not added to staging area")
	}
}

func TestRebase(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	newBranchName := "a-new-branch"
	if configDefaultBranchName, err := getConfigDefaultBranchName(); err != nil {
		t.Fatal(err)
	} else if err := commitBlankFile("Z"); err != nil {
		t.Fatal(err)
	} else if err := CreateAndSwitchToBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if err := commitBlankFile("Y"); err != nil {
		t.Fatal(err)
	} else if err := Rebase(configDefaultBranchName, newBranchName); err != nil {
		t.Fatal(err)
	} else if _, err := os.Stat("Z"); err != nil {
		t.Fatal(err)
	}
}

func TestLog(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	hashes := []string{}
	for _, ref := range k_RefNames {
		hash, err := RevParse(ref)
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, hash)
	}

	output, err := Log("--reverse", "--format='%H\n%s'")
	if err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(strings.NewReader(output))
	for i := 0; i < len(hashes); i++ {
		if hash, err := r.ReadString('\n'); err != nil {
			t.Fatal(err)
		} else {
			expectEq(t, hashes[i], strings.TrimSpace(hash))
		}

		if subject, err := r.ReadString('\n'); err != nil {
			if err != io.EOF || len(subject) == 0 {
				t.Fatal(err)
			}
		} else {
			expectEq(t, k_CommitDescriptions[i], strings.TrimSpace(subject))
		}
	}
}

func TestGetForkPoint(t *testing.T) {
	cleanup := setupGitRepo(t)
	defer cleanup()

	newBranchName := "divergence"
	if hash, err := RevParse("HEAD"); err != nil {
		t.Fatal(err)
	} else if configDefaultBranchName, err := getConfigDefaultBranchName(); err != nil {
		t.Fatal(err)
	} else if err := CreateAndSwitchToBranch(newBranchName); err != nil {
		t.Fatal(err)
	} else if err := commitBlankFile("Z"); err != nil {
		t.Fatal(err)
	} else if forkHash, err := GetForkPoint(configDefaultBranchName); err != nil {
		t.Fatal(err)
	} else if forkHash2, err := GetForkPoint(configDefaultBranchName, newBranchName); err != nil {
		// also test the 2-arg version
		t.Fatal(err)
	} else {
		expectEq(t, hash, forkHash)
		expectEq(t, hash, forkHash2)
	}

	if _, err := GetForkPoint("doesnt-exist"); err == nil {
		t.Fatal("Expected error for invalid ref")
	}
}

func TestGetPushRemoteForBranch(t *testing.T) {
	func() {
		cleanup := setupGitRepo(t)
		defer cleanup()

		const k_PushBranchName = "push-to-me"
		if err := CreateBranch(k_PushBranchName); err != nil {
			t.Fatal(err)
		} else if configDefaultBranchName, err := getConfigDefaultBranchName(); err != nil {
			t.Fatal(err)
		} else if err := git("branch", "-u", k_PushBranchName); err != nil {
			t.Fatal(err)
		} else if remoteName, err := GetPushRemoteForBranch(configDefaultBranchName); err != nil {
			t.Fatal(err)
		} else {
			expectEq(t, ".", remoteName)
		}
	}()

	func() {
		cleanup := setupGitRepo(t)
		defer cleanup()

		const k_DummyRemoteName = "dummy"

		configDefaultBranchName, err := getConfigDefaultBranchName()
		if err != nil {
			t.Fatal(err)
		}
		var configPath = fmt.Sprintf("branch.%s.pushRemote", configDefaultBranchName)

		if err := git("config", "--add", configPath, k_DummyRemoteName); err != nil {
			t.Fatal(err)
		} else if remoteName, err := GetPushRemoteForBranch(configDefaultBranchName); err != nil {
			t.Fatal(err)
		} else {
			expectEq(t, k_DummyRemoteName, remoteName)
		}
	}()
}
