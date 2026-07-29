package main

import (
	"crypto/sha1"
	"fmt"
)

// namespace is the standard DNS namespace UUID from RFC 4122
// (6ba7b810-9dad-11d1-80b4-00c04fd430c8), used to salt every generated
// name-based UUID. It must stay constant: changing it changes every UUID
// this function produces for the same input word.
var namespace = [16]byte{
	0x6b, 0xa7, 0xb8, 0x10, 0x9d, 0xad, 0x11, 0xd1,
	0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8,
}

// GenerateUUID returns a version-5 (SHA-1, name-based) UUID for word.
// The same word always produces the same UUID.
func GenerateUUID(word string) string {
	h := sha1.New()
	h.Write(namespace[:])
	h.Write([]byte(word))
	sum := h.Sum(nil)

	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50 // version 5
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
