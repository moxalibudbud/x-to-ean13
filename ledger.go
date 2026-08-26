package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
)

// Ledger is a persistent, append-only allocation table mapping arbitrary keys
// (e.g. a product name) to EAN-13 codes built from a single company prefix.
//
// A GTIN cannot be safely derived by hashing a key: uniqueness is a property
// of everything ever issued, which a pure function has no way to know. Ledger
// instead hands out the next unused item reference and remembers the
// assignment, which is the only way to guarantee both properties GS1 actually
// requires: no two keys ever share a code, and a given key always gets the
// same code back.
type Ledger struct {
	path   string
	prefix string

	byKey map[string]string // key -> ean13
	used  map[uint64]bool   // item references already issued

	cursor   uint64 // next item reference to try
	capacity uint64 // one past the largest usable item reference
}

// ledgerHeader matches the (word, ean13) shape of the tool's output CSV, so an
// existing output file can be copied in verbatim to seed the ledger.
var ledgerHeader = []string{"word", "ean13"}

// LoadLedger reads the ledger at path. A missing file yields an empty ledger
// for prefix. An existing file is rejected if it contains a duplicate key, a
// duplicate code, a code that fails ValidateEAN13, or a code that was not
// built from prefix (which usually means the ledger belongs to a different
// -prefix run).
func LoadLedger(path, prefix string) (*Ledger, error) {
	if err := ValidateCompanyPrefix(prefix); err != nil {
		return nil, err
	}

	l := &Ledger{
		path:     path,
		prefix:   prefix,
		byKey:    make(map[string]string),
		used:     make(map[uint64]bool),
		capacity: ItemRefCapacity(prefix),
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening ledger %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err == io.EOF {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading ledger %s: %w", path, err)
	}
	if len(header) < 2 || header[0] == "" || header[1] != "ean13" {
		return nil, fmt.Errorf("ledger %s: unexpected header %v, want columns (<word>, ean13)", path, header)
	}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading ledger %s: %w", path, err)
		}
		if len(record) < 2 {
			return nil, fmt.Errorf("ledger %s: malformed row %v", path, record)
		}

		key, code := record[0], record[1]
		if _, exists := l.byKey[key]; exists {
			return nil, fmt.Errorf("ledger %s: duplicate key %q", path, key)
		}
		if !ValidateEAN13(code) {
			return nil, fmt.Errorf("ledger %s: %q has an invalid check digit for key %q", path, code, key)
		}
		itemRef, err := ItemRefOf(prefix, code)
		if err != nil {
			return nil, fmt.Errorf("ledger %s: code %q for key %q: %w", path, code, key, err)
		}
		if l.used[itemRef] {
			return nil, fmt.Errorf("ledger %s: duplicate code %q", path, code)
		}

		l.byKey[key] = code
		l.used[itemRef] = true
	}

	return l, nil
}

// Allocate returns the EAN-13 for key, generating and recording a new one if
// key has not been seen before. isNew reports whether a new code was minted.
func (l *Ledger) Allocate(key string) (ean13 string, isNew bool, err error) {
	if code, exists := l.byKey[key]; exists {
		return code, false, nil
	}

	for l.cursor < l.capacity && l.used[l.cursor] {
		l.cursor++
	}
	if l.cursor >= l.capacity {
		return "", false, fmt.Errorf(
			"company prefix %q is exhausted: all %d item references are in use", l.prefix, l.capacity,
		)
	}

	code, err := BuildEAN13(l.prefix, l.cursor)
	if err != nil {
		return "", false, err
	}

	l.byKey[key] = code
	l.used[l.cursor] = true
	l.cursor++

	return code, true, nil
}

// Save writes the ledger to its path atomically: it writes to a temp file
// first and renames it into place, so a crash mid-write cannot corrupt an
// existing ledger.
func (l *Ledger) Save() error {
	tmpPath := l.path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", tmpPath, err)
	}

	w := csv.NewWriter(f)
	if err := w.Write(ledgerHeader); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing ledger header: %w", err)
	}
	keys := make([]string, 0, len(l.byKey))
	for key := range l.byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := w.Write([]string{key, l.byKey[key]}); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("writing ledger row for %q: %w", key, err)
		}
	}
	w.Flush()

	if err := w.Error(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("flushing ledger: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, l.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming %s to %s: %w", tmpPath, l.path, err)
	}

	return nil
}
