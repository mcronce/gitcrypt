package main
import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"
)

const ITERATIONS_PER_TIMESTAMP = 65535

type Commit struct {
	Text []byte
	Hash []byte
}

// WARNING: HERE BE DRAGONS
func str_to_bytes(str string) []byte /* {{{ */ {
	string_header := (*reflect.StringHeader)(unsafe.Pointer(&str))
    bytes_header := &reflect.SliceHeader{
        Data : string_header.Data,
        Len : string_header.Len,
        Cap : string_header.Len,
    }
    return *(*[]byte)(unsafe.Pointer(bytes_header))
} // }}}

// WARNING: HERE BE DRAGONS
func bytes_to_str(bytes []byte) string /* {{{ */ {
    bytes_header := (*reflect.SliceHeader)(unsafe.Pointer(&bytes))
    string_header := &reflect.StringHeader{
        Data : bytes_header.Data,
        Len : bytes_header.Len,
    }
    return *(*string)(unsafe.Pointer(string_header))
} // }}}

// WARNING: HERE BE (admittedly smaller) DRAGONS.
func get_goroutine_id_hash() []byte /* {{{ */ {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	sum := md5.Sum(b)
	return []byte(hex.EncodeToString(sum[:]))
} // }}}

func replace_section(bytes []byte, pos int, repl []byte) /* {{{ */ {
	end_pos := len(repl)
	for i := 0; i < end_pos; i++ {
		bytes[i + pos] = repl[i]
	}
} // }}}

// WARNING:  HERE BE DRAGONS
func uint16_as_bytes(i *uint16) []byte /* {{{ */ {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(i)),
		Len: 2,
		Cap: 2,
	}))
} // }}}

// WARNING:  HERE BE DRAGONS
func int64_as_bytes(i *int64) []byte /* {{{ */ {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(i)),
		Len: 8,
		Cap: 8,
	}))
} // }}}

var int_md5s [ITERATIONS_PER_TIMESTAMP][]byte
func cache_int_md5s() /* {{{ */ {
	var i uint16
	i_bytes := uint16_as_bytes(&i)
	for i = 0; i < ITERATIONS_PER_TIMESTAMP; i++ {
		int_checksum := md5.Sum(i_bytes)
		int_md5s[i] = make([]byte, md5.Size * 2)
		hex.Encode(int_md5s[i], int_checksum[:])
	}
} // }}}

var hashes uint64
func find_commit_that_works(commit_prefix []byte, commit_channel chan<- *Commit, terminate_channel <-chan struct{}) {
	zero := byte(0)

	commit := append(commit_prefix, get_goroutine_id_hash()...)
	commit = append(commit, byte(' '))
	nanosec_md5_position := len(commit)
	commit = append(commit, make([]byte, md5.Size * 2, byte('0'))...)
	commit = append(commit, byte(' '))
	int_md5_position := len(commit)
	commit = append(commit, make([]byte, md5.Size * 2, byte('0'))...)
	commit = append(commit, byte('\n'))
	commit_header := []byte(fmt.Sprintf("commit %d\000", len(commit)))
	nanosec_md5_position = nanosec_md5_position + len(commit_header)
	int_md5_position = int_md5_position + len(commit_header)
	commit = append(commit_header, commit...)

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
				fmt.Printf("%x %x %s %d\n", sha, nano_checksum, int_md5s[i], i)
				if(sha[2] == zero) {
					commit_channel <- &Commit{
						Text: commit[len(commit_header):],
						Hash: sha[:],
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

func get_git_timestamp() string /* {{{ */ {
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
} // }}}

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

	timestamp := get_git_timestamp()
	cache_int_md5s()
	message := flag.Arg(0)

	commit_prefix := []byte(fmt.Sprintf("tree %s\nparent %s\nauthor %s <%s> %s\ncommitter %s <%s> %s\n\n%s\n\n\nTo pass the absurd cryptographic restriction, I have appended these hashes:  ",
		strings.TrimSpace(string(git_write_tree)),
		strings.TrimSpace(string(git_rev_parse)),
		strings.TrimSpace(string(git_user_name)), strings.TrimSpace(string(git_user_email)), timestamp,
		strings.TrimSpace(string(git_user_name)), strings.TrimSpace(string(git_user_email)), timestamp,
		message,
	))
	commit_channel := make(chan *Commit)
	terminate_channel := make(chan struct{})

	hashes = 0
	start := time.Now()
	for i := 0; i < runtime.GOMAXPROCS(-1); i++ {
		go find_commit_that_works(commit_prefix, commit_channel, terminate_channel)
	}

	commit := <-commit_channel
	elapsed := time.Now().Sub(start).Seconds()
	fmt.Printf("--- Calculated %d hashes in %f seconds, for a total of %f hashes /sec\n", atomic.LoadUint64(&hashes), elapsed, float64(atomic.LoadUint64(&hashes)) / elapsed)

	our_commit_hash := hex.EncodeToString(commit.Hash)
	fmt.Printf("--- Found a thing that works (sha1 %s):\n", our_commit_hash)
	for _, line := range strings.Split(string(commit.Text), "\n") {
		fmt.Printf("\t%s\n", line)
	}

	close(terminate_channel)

	git_hash_object := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	f, err := git_hash_object.StdinPipe()
	if(err != nil) {
		fmt.Printf("!!! %s\n", err)
		os.Exit(2)
	}
	go func() {
		defer f.Close()
		f.Write(commit.Text)
	}()
	git_commit_hash, err := git_hash_object.CombinedOutput()
	if(err != nil) {
		fmt.Printf("!!! %s\n%s\n", err, git_commit_hash)
		os.Exit(3)
	}
	fmt.Printf("--- `git hash-object` result: %s\n", git_commit_hash)

	if(string(git_commit_hash[:40]) != our_commit_hash) {
		fmt.Printf("!!! Hash mismatch (%s vs %s)\n", string(git_commit_hash[:40]), string(our_commit_hash))
		os.Exit(4)
	}

	_, err = exec.Command("git", "reset", "--hard", our_commit_hash).Output()
	if(err != nil) {
		fmt.Printf("!!! %s\n", err)
		os.Exit(5)
	}
}

