package gitcrypt
import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"reflect"
	"runtime"
	"unsafe"
)

func replace_section(bytes []byte, pos int, repl []byte) /* {{{ */ {
	end_pos := len(repl)
	for i := 0; i < end_pos; i++ {
		bytes[i + pos] = repl[i]
	}
} // }}}

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

// WARNING: HERE BE (admittedly smaller) DRAGONS.
func get_goroutine_id_hash() []byte /* {{{ */ {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	sum := md5.Sum(b)
	return []byte(hex.EncodeToString(sum[:]))
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

