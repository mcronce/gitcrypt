package main
import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mcronce/gitcrypt/pkg/gitcrypt"
)

func main() {
	flag.Parse()
	if(flag.NArg() != 1) {
		print("usage: commit 'message'\n")
		os.Exit(1)
	}

	git_write_tree, err := exec.Command("git", "write-tree").Output()
	if(err != nil) {
		fmt.Printf("!!! %s\n", err)
		os.Exit(2)
	}

	git_rev_parse, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if(err != nil) {
		fmt.Printf("!!! %s\n", err)
		os.Exit(2)
	}

	git_user_name, err := exec.Command("git", "config", "--get", "user.name").Output()
	if(err != nil) {
		fmt.Printf("!!! %s\n", err)
		os.Exit(2)
	}

	git_user_email, err := exec.Command("git", "config", "--get", "user.email").Output()
	if(err != nil) {
		fmt.Printf("!!! %s\n", err)
		os.Exit(2)
	}

	timestamp := gitcrypt.GetGitTimestamp()
	message := flag.Arg(0)

	commit_prefix := gitcrypt.MakeCommitPrefix(
		strings.TrimSpace(string(git_write_tree)),
		strings.TrimSpace(string(git_rev_parse)),
		gitcrypt.MakeFullUser(strings.TrimSpace(string(git_user_name)), strings.TrimSpace(string(git_user_email))), timestamp,
		gitcrypt.MakeFullUser(strings.TrimSpace(string(git_user_name)), strings.TrimSpace(string(git_user_email))), timestamp,
		message,
	)

	commit := gitcrypt.FindCommitThatWorks(commit_prefix)

	fmt.Printf("--- Found a thing that works (sha1 %s):\n", commit.Hash)
	for _, line := range strings.Split(string(commit.Text), "\n") {
		fmt.Printf("\t%s\n", line)
	}

	err = gitcrypt.WriteCommit(commit)
	if(err != nil) {
		fmt.Printf("!!! %s\n", err)
		os.Exit(3)
	}
}

