package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// newTestLedger returns an empty ledger backed by a file in a fresh temp
// directory, for tests that only need Allocate semantics and don't care about
// persistence.
func newTestLedger(t *testing.T, prefix string) *Ledger {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.csv")
	l, err := LoadLedger(path, prefix)
	if err != nil {
		t.Fatalf("LoadLedger(%q, %q) unexpected error: %v", path, prefix, err)
	}
	return l
}

func TestLedger_AllocateIsStableForSameKey(t *testing.T) {
	l := newTestLedger(t, "200")

	first, isNew, err := l.Allocate("widget")
	if err != nil || !isNew {
		t.Fatalf("first Allocate(\"widget\") = (%q, %v, %v), want a new code", first, isNew, err)
	}

	second, isNew, err := l.Allocate("widget")
	if err != nil {
		t.Fatalf("second Allocate(\"widget\") unexpected error: %v", err)
	}
	if isNew {
		t.Error("second Allocate(\"widget\") reported isNew=true, want false")
	}
	if second != first {
		t.Errorf("Allocate(\"widget\") = %q then %q, want the same code both times", first, second)
	}
}

func TestLedger_AllocateGivesDistinctKeysDistinctCodes(t *testing.T) {
	l := newTestLedger(t, "200")

	a, _, err := l.Allocate("apple")
	if err != nil {
		t.Fatalf("Allocate(\"apple\") unexpected error: %v", err)
	}
	b, _, err := l.Allocate("banana")
	if err != nil {
		t.Fatalf("Allocate(\"banana\") unexpected error: %v", err)
	}
	if a == b {
		t.Errorf("Allocate(\"apple\") and Allocate(\"banana\") both = %q, want distinct codes", a)
	}
}

func TestLedger_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.csv")

	l1, err := LoadLedger(path, "200")
	if err != nil {
		t.Fatalf("LoadLedger unexpected error: %v", err)
	}
	apple, _, err := l1.Allocate("apple")
	if err != nil {
		t.Fatalf("Allocate(\"apple\") unexpected error: %v", err)
	}
	banana, _, err := l1.Allocate("banana")
	if err != nil {
		t.Fatalf("Allocate(\"banana\") unexpected error: %v", err)
	}
	if err := l1.Save(); err != nil {
		t.Fatalf("Save unexpected error: %v", err)
	}

	l2, err := LoadLedger(path, "200")
	if err != nil {
		t.Fatalf("LoadLedger (reload) unexpected error: %v", err)
	}

	gotApple, isNew, err := l2.Allocate("apple")
	if err != nil || isNew || gotApple != apple {
		t.Errorf("reloaded Allocate(\"apple\") = (%q, %v, %v), want (%q, false, nil)", gotApple, isNew, err, apple)
	}
	gotBanana, isNew, err := l2.Allocate("banana")
	if err != nil || isNew || gotBanana != banana {
		t.Errorf("reloaded Allocate(\"banana\") = (%q, %v, %v), want (%q, false, nil)", gotBanana, isNew, err, banana)
	}

	cherry, isNew, err := l2.Allocate("cherry")
	if err != nil || !isNew {
		t.Fatalf("reloaded Allocate(\"cherry\") = (%q, %v, %v), want a new code", cherry, isNew, err)
	}
	if cherry == apple || cherry == banana {
		t.Errorf("reloaded Allocate(\"cherry\") = %q, collides with a seeded code", cherry)
	}
}

func TestLedger_AllocationSkipsSeededValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.csv")
	seed, err := BuildEAN13("200", 0)
	if err != nil {
		t.Fatalf("BuildEAN13 setup failed: %v", err)
	}
	writeLedgerFile(t, path, [][2]string{{"existing", seed}})

	l, err := LoadLedger(path, "200")
	if err != nil {
		t.Fatalf("LoadLedger unexpected error: %v", err)
	}

	got, isNew, err := l.Allocate("new-item")
	if err != nil {
		t.Fatalf("Allocate(\"new-item\") unexpected error: %v", err)
	}
	if !isNew {
		t.Error("Allocate(\"new-item\") reported isNew=false, want true")
	}
	if got == seed {
		t.Errorf("Allocate(\"new-item\") = %q, collides with pre-seeded code", got)
	}
}

func TestLedger_ExhaustionErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.csv")
	// An 11-digit prefix leaves exactly 1 item-reference digit: 10 codes total.
	prefix := "12345678901"

	l, err := LoadLedger(path, prefix)
	if err != nil {
		t.Fatalf("LoadLedger unexpected error: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, _, err := l.Allocate(fmt.Sprintf("key-%d", i)); err != nil {
			t.Fatalf("Allocate(key-%d) unexpected error: %v", i, err)
		}
	}

	if _, _, err := l.Allocate("one-too-many"); err == nil {
		t.Error("Allocate on an exhausted prefix = nil error, want error")
	}
}

func TestLedger_LoadRejectsPrefixMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.csv")
	code, err := BuildEAN13("200", 1)
	if err != nil {
		t.Fatalf("BuildEAN13 setup failed: %v", err)
	}
	writeLedgerFile(t, path, [][2]string{{"widget", code}})

	if _, err := LoadLedger(path, "250"); err == nil {
		t.Error("LoadLedger with a mismatched prefix = nil error, want error")
	}
}

func TestLedger_LoadRejectsDuplicateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.csv")
	c1, _ := BuildEAN13("200", 1)
	c2, _ := BuildEAN13("200", 2)
	writeLedgerFile(t, path, [][2]string{{"widget", c1}, {"widget", c2}})

	if _, err := LoadLedger(path, "200"); err == nil {
		t.Error("LoadLedger with a duplicate key = nil error, want error")
	}
}

func TestLedger_LoadRejectsDuplicateCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.csv")
	code, _ := BuildEAN13("200", 1)
	writeLedgerFile(t, path, [][2]string{{"widget", code}, {"gadget", code}})

	if _, err := LoadLedger(path, "200"); err == nil {
		t.Error("LoadLedger with a duplicate code = nil error, want error")
	}
}

func TestLedger_LoadRejectsBadCheckDigit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.csv")
	writeLedgerFile(t, path, [][2]string{{"widget", "2000000000009"}})

	if _, err := LoadLedger(path, "200"); err == nil {
		t.Error("LoadLedger with an invalid check digit = nil error, want error")
	}
}

// TestLedger_NoCollisionsAtScale is the direct regression test for the
// original hash-and-mod scheme's collision problem: with a 10^8 item
// reference space, the birthday bound predicts a >99% chance of at least one
// collision among 50,000 hashed keys. Sequential allocation must produce zero.
func TestLedger_NoCollisionsAtScale(t *testing.T) {
	l := newTestLedger(t, "200")
	seen := make(map[string]bool)

	const n = 50000
	for i := 0; i < n; i++ {
		code, _, err := l.Allocate(fmt.Sprintf("item-%d", i))
		if err != nil {
			t.Fatalf("Allocate(item-%d) unexpected error: %v", i, err)
		}
		if seen[code] {
			t.Fatalf("Allocate produced duplicate code %q at item-%d", code, i)
		}
		seen[code] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d distinct codes, want %d", len(seen), n)
	}
}

func writeLedgerFile(t *testing.T, path string, rows [][2]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating ledger fixture: %v", err)
	}
	defer f.Close()

	fmt.Fprintln(f, "word,ean13")
	for _, row := range rows {
		fmt.Fprintf(f, "%s,%s\n", row[0], row[1])
	}
}
