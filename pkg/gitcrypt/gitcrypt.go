package gitcrypt
import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

const ITERATIONS_PER_TIMESTAMP = math.MaxUint16
const OUR_MESSAGE = "\n\n\nTo pass the absurd cryptographic restriction, I have appended these hashes:"

type Commit struct {
	Text []byte
	Hash string
}

var re_commit *regexp.Regexp
var zero byte
func init() {
	zero = byte(0)
	re_commit = regexp.MustCompile("^tree ([0-9a-f]+)\nparent ([0-9a-f]+)\nauthor ([^<]+ <[^>]+>) ([0-9 +-]+)\ncommitter ([^<]+ <[^>]+>) ([0-9 +-]+)\n\n(?s:(.+))")
}

func build_whole_commit(main_commit []byte) ([]byte, int, int, int) {
	commit := append(main_commit, get_goroutine_id_hash()...)
	commit = append(commit, byte(' '))
	nanosec_md5_position := len(commit)
	commit = append(commit, make([]byte, md5.Size * 2, byte('0'))...)
	commit = append(commit, byte(' '))
	int_md5_position := len(commit)
	commit = append(commit, make([]byte, md5.Size * 2, byte('0'))...)
	commit = append(commit, byte('\n'))
	commit_header := []byte(fmt.Sprintf("commit %d\000", len(commit)))
	commit_header_length := len(commit_header)
	nanosec_md5_position = nanosec_md5_position + commit_header_length
	int_md5_position = int_md5_position + commit_header_length
	commit = append(commit_header, commit...)

	return commit, commit_header_length, nanosec_md5_position, int_md5_position
}

var hashes uint64
func commit_message_worker(commit_prefix []byte, commit_channel chan<- *Commit, terminate_channel <-chan struct{}) {
	commit, commit_header_length, nanosec_md5_position, int_md5_position := build_whole_commit(commit_prefix)

	var i uint16
	var nanos int64
	nano_bytes := int64_as_bytes(&nanos)
	for {
		nanos = time.Now().UnixNano()
		nano_checksum := md5.Sum(nano_bytes)
		hex.Encode(commit[nanosec_md5_position:], nano_checksum[:])
		for i = 0; i < ITERATIONS_PER_TIMESTAMP; i++ {
			copy(commit[int_md5_position:], int_md5s[i])
			sha := sha1.Sum(commit)
			atomic.AddUint64(&hashes, 1)
			if(sha[0] == zero && sha[1] == zero) {
				if(sha[2] == zero) {
					commit_channel <- &Commit{
						Text: commit[commit_header_length:],
						Hash: hex.EncodeToString(sha[:]),
					}
					return
				}
			}
		}
		select {
			case <-terminate_channel:
				return
			default:
				// Next loop iteration
		}
	}
}

func GetGitTimestamp() string {
	now := time.Now()
	_, offset := now.Zone()
	// Going from seconds to hours/minutes offset is a shit show, but at least it only has to happen once 
	sign := "+"
	if(offset < 0) {
		sign = "-"
	}
	offset = int(math.Abs(float64(offset)))
	hours_part := int(offset / 3600)
	minutes_part := int((offset % 3600) / 60)
	return fmt.Sprintf("%d %s%02d%02d", now.Unix(), sign, hours_part, minutes_part)
}

func FindCommitThatWorks(in_commit []byte) *Commit {
	start := time.Now()
	cache_int_md5s()
	elapsed := time.Now().Sub(start).Seconds()
	fmt.Printf("--- Caching %d MD5 checksums took %f seconds\n", ITERATIONS_PER_TIMESTAMP, elapsed)
	commit_channel := make(chan *Commit)
	terminate_channel := make(chan struct{})

	hashes = 0
	for i := 0; i < runtime.GOMAXPROCS(-1); i++ {
		go commit_message_worker(in_commit, commit_channel, terminate_channel)
	}

	commit := <-commit_channel
	elapsed = time.Now().Sub(start).Seconds()
	fmt.Printf("--- Calculated %d hashes in %f seconds, for a total of %f hashes /sec\n", atomic.LoadUint64(&hashes), elapsed, float64(atomic.LoadUint64(&hashes)) / elapsed)

	close(terminate_channel)
	return commit
}

func MakeFullUser(name string, email string) string {
	return fmt.Sprintf("%s <%s>", name, email)
}

func MakeCommitPrefix(tree string, parent string, author string, author_time string, committer string, committer_time string, message string) []byte {
	return []byte(fmt.Sprintf("tree %s\nparent %s\nauthor %s %s\ncommitter %s %s\n\n%s%s  ",
		tree,
		parent,
		author, author_time,
		committer, committer_time,
		message,
		OUR_MESSAGE,
	))
}

func ParseCommit(commit string) (bool, string, string, string, string, string, string, string) {
		matches := re_commit.FindStringSubmatch(commit)
		if(matches == nil) {
			return false, "", "", "", "", "", "", ""
		}

		i := strings.Index(matches[7], OUR_MESSAGE)
		if(i == -1) {
			return true, matches[1], matches[2], matches[3], matches[4], matches[5], matches[6], matches[7]
		}

		return true, matches[1], matches[2], matches[3], matches[4], matches[5], matches[6], matches[7][:i]
}

func WriteCommit(commit *Commit) error {
	git_hash_object := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	f, err := git_hash_object.StdinPipe()
	if(err != nil) {
		return err
	}
	go func() {
		defer f.Close()
		f.Write(commit.Text)
	}()
	git_commit_hash, err := git_hash_object.CombinedOutput()
	if(err != nil) {
		return errors.New(fmt.Sprintf("%s (%s)", err, git_commit_hash))
	}

	if(string(git_commit_hash[:40]) != commit.Hash) {
		return errors.New(fmt.Sprintf("Hash mismatch (%s vs %s)", string(git_commit_hash[:40]), commit.Hash))
	}

	_, err = exec.Command("git", "reset", "--hard", commit.Hash).Output()
	if(err != nil) {
		return err
	}
	return nil
}

