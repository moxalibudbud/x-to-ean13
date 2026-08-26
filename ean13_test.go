package main

import (
	"strings"
	"testing"
)

func TestCalculateEAN13CheckDigit_KnownVectors(t *testing.T) {
	cases := []struct {
		code12 string
		want   string
	}{
		{"400638133393", "1"}, // 4006381333931
		{"978030640615", "7"}, // 9780306406157
		{"000000000000", "0"},
	}
	for _, c := range cases {
		got, err := calculateEAN13CheckDigit(c.code12)
		if err != nil {
			t.Errorf("calculateEAN13CheckDigit(%q) unexpected error: %v", c.code12, err)
			continue
		}
		if got != c.want {
			t.Errorf("calculateEAN13CheckDigit(%q) = %q, want %q", c.code12, got, c.want)
		}
	}
}

func TestValidateCompanyPrefix(t *testing.T) {
	valid := []string{"2", "200", "0614141", "12345678901"} // up to 11 digits
	for _, p := range valid {
		if err := ValidateCompanyPrefix(p); err != nil {
			t.Errorf("ValidateCompanyPrefix(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{"", "abc", "20a", "123456789012"} // 12 digits leaves no item ref room
	for _, p := range invalid {
		if err := ValidateCompanyPrefix(p); err == nil {
			t.Errorf("ValidateCompanyPrefix(%q) = nil, want error", p)
		}
	}
}

// Regression test for the panic in the original (companyPrefix + seq + "0")[:12]
// implementation, which paniced on any prefix shorter than 3 digits.
func TestBuildEAN13_ShortPrefixDoesNotPanic(t *testing.T) {
	code, err := BuildEAN13("2", 0)
	if err != nil {
		t.Fatalf("BuildEAN13(\"2\", 0) unexpected error: %v", err)
	}
	if len(code) != 13 {
		t.Fatalf("BuildEAN13(\"2\", 0) = %q, want 13 digits", code)
	}
	if !ValidateEAN13(code) {
		t.Fatalf("BuildEAN13(\"2\", 0) = %q, want a valid check digit", code)
	}
}

// Regression test for the original silent truncation: a 7-digit company
// prefix only leaves 5 digits of item reference, so a 6-digit item reference
// must be rejected rather than silently truncated.
func TestBuildEAN13_OverflowingItemRefErrors(t *testing.T) {
	if _, err := BuildEAN13("0614141", 100000); err == nil {
		t.Fatalf("BuildEAN13(\"0614141\", 100000) = nil error, want error (6 digits does not fit in 5)")
	}

	code, err := BuildEAN13("0614141", 99999)
	if err != nil {
		t.Fatalf("BuildEAN13(\"0614141\", 99999) unexpected error: %v", err)
	}
	if !strings.HasPrefix(code, "0614141") || len(code) != 13 {
		t.Fatalf("BuildEAN13(\"0614141\", 99999) = %q, want 13-digit code starting with 0614141", code)
	}
}

func TestClassifyPrefix(t *testing.T) {
	cases := []struct {
		code string
		want PrefixClass
	}{
		{"200", PrefixRestricted},
		{"2001234567890", PrefixRestricted},
		{"020", PrefixRestricted},
		{"045", PrefixRestricted},
		{"978", PrefixReserved},
		{"990", PrefixReserved},
		{"400", PrefixLicensable},
		{"061414112345", PrefixLicensable},
	}
	for _, c := range cases {
		if got := ClassifyPrefix(c.code); got != c.want {
			t.Errorf("ClassifyPrefix(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestValidateGTIN13(t *testing.T) {
	restricted, err := BuildEAN13("200", 1)
	if err != nil {
		t.Fatalf("BuildEAN13 setup failed: %v", err)
	}
	if err := ValidateGTIN13(restricted); err != nil {
		t.Errorf("ValidateGTIN13(%q) = %v, want nil (restricted distribution is valid for internal use)", restricted, err)
	}

	if err := ValidateGTIN13("0000000000000"); err == nil {
		t.Error("ValidateGTIN13(\"0000000000000\") = nil, want error (all-zero payload)")
	}

	reserved, err := BuildEAN13("978", 1)
	if err != nil {
		t.Fatalf("BuildEAN13 setup failed: %v", err)
	}
	if err := ValidateGTIN13(reserved); err == nil {
		t.Errorf("ValidateGTIN13(%q) = nil, want error (reserved prefix)", reserved)
	}
}

func TestGetOrGenerateEAN13_RejectsInvalidSupplied(t *testing.T) {
	ledger := newTestLedger(t, "200")
	bad := "0000000000000"

	if _, _, err := GetOrGenerateEAN13(&bad, "widget", ledger); err == nil {
		t.Error("GetOrGenerateEAN13 with an invalid supplied code = nil error, want error")
	}
}

func TestGetOrGenerateEAN13_KeepsValidSupplied(t *testing.T) {
	ledger := newTestLedger(t, "200")
	good, err := BuildEAN13("200", 42)
	if err != nil {
		t.Fatalf("BuildEAN13 setup failed: %v", err)
	}

	got, isNew, err := GetOrGenerateEAN13(&good, "widget", ledger)
	if err != nil {
		t.Fatalf("GetOrGenerateEAN13 unexpected error: %v", err)
	}
	if isNew {
		t.Error("GetOrGenerateEAN13 reported isNew=true for a supplied code")
	}
	if got != good {
		t.Errorf("GetOrGenerateEAN13 = %q, want %q", got, good)
	}
}

func TestGetOrGenerateEAN13_AllocatesWhenMissing(t *testing.T) {
	ledger := newTestLedger(t, "200")

	got, isNew, err := GetOrGenerateEAN13(nil, "widget", ledger)
	if err != nil {
		t.Fatalf("GetOrGenerateEAN13 unexpected error: %v", err)
	}
	if !isNew {
		t.Error("GetOrGenerateEAN13 reported isNew=false for a fresh key")
	}
	if !ValidateEAN13(got) {
		t.Errorf("GetOrGenerateEAN13 = %q, want a valid EAN-13", got)
	}
}
