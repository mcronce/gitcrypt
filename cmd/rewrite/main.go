package main
import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mcronce/gitcrypt/pkg/gitcrypt"
)

type MapStringString map[string]string

func (this *MapStringString) String() string {
	return fmt.Sprintf("%v", *this)
}

func (this *MapStringString) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if(len(parts) < 2) {
		return errors.New("map[string]string flags must be of the format \"--flag=key1=value1 --flag=key2=value2\"")
	}
	fmt.Printf("Replacing %s with %s\n", parts[0], parts[1])
	(*this)[parts[0]] = parts[1]
	return nil
}

var replace_authors MapStringString
var replace_committers MapStringString

func init() {
	replace_authors = make(MapStringString)
	replace_committers = make(MapStringString)
	flag.Var(&replace_authors, "replace-author", "Replace this author (format \"Author Name <author@example.com>\") with that author")
	flag.Var(&replace_committers, "replace-committer", "Replace this committer (format \"Committer Name <committer@example.com>\") with that author")
}

func main() {
	flag.Parse()
	if(flag.NArg() != 1) {
		print("usage: rewrite 'revision-to-go-back-to (exclusive)'\n")
		os.Exit(1)
	}

	fmt.Printf("%v\n%v\n", replace_authors, replace_committers)

	start := flag.Arg(0)
	git_rev_parse, err := exec.Command("git", "rev-parse", start).CombinedOutput()
	if(err != nil) {
		fmt.Printf("!!! %s\n%s\n", err, git_rev_parse)
		os.Exit(2)
	}
	start_rev := strings.TrimSpace(string(git_rev_parse))

	fmt.Printf("--- Starting with %s (%s)\n", start, start_rev)

	git_rev_list, err := exec.Command("git", "rev-list", "--reverse", fmt.Sprintf("%s..HEAD", start_rev)).CombinedOutput()
	if(err != nil) {
		fmt.Printf("!!! %s\n%s\n", err, git_rev_list)
		os.Exit(2)
	}
	fmt.Printf("---------\n%s\n---------\n", git_rev_list)
	revisions := strings.Split(strings.TrimSpace(string(git_rev_list)), "\n")

	git_reset_start, err := exec.Command("git", "reset", "--hard", start_rev).CombinedOutput()
	if(err != nil) {
		fmt.Printf("!!! %s\n%s\n", err, git_reset_start)
		os.Exit(2)
	}
	fmt.Printf("--- %s\n", git_reset_start)

	new_parent := start_rev
	for i, rev := range revisions {
		fmt.Printf("--- Working on revision %s (%d)\n", rev, i)
		git_cat_file, err := exec.Command("git", "cat-file", "-p", rev).Output()
		if(err != nil) {
			fmt.Printf("!!! %s\n%s\n", err, git_cat_file)
			os.Exit(3)
		}

		matching, tree, _, author, author_time, committer, committer_time, message := gitcrypt.ParseCommit(string(git_cat_file))
		if(!matching) {
			fmt.Printf("!!! Commit %s did not parse:\n\n%s\n", rev, git_cat_file)
			os.Exit(4)
		}

		new_author, ok := replace_authors[author]
		if(!ok) {
			new_author = author
		}

		new_committer, ok := replace_committers[committer]
		if(!ok) {
			new_committer = committer
		}

		new_commit_prefix := gitcrypt.MakeCommitPrefix(tree, new_parent, new_author, author_time, new_committer, committer_time, message)
		new_commit := gitcrypt.FindCommitThatWorks(new_commit_prefix)

		fmt.Printf("--- Found a thing that works (sha1 %s)\n", new_commit.Hash)
		new_parent = new_commit.Hash

		err = gitcrypt.WriteCommit(new_commit)
		if(err != nil) {
			fmt.Printf("!!! %s\n", err)
			os.Exit(3)
		}
	}
}

