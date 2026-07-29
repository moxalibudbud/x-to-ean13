package main

import "testing"

func TestGenerateUUID_KnownVector(t *testing.T) {
	// Reference value from Python's uuid.uuid5(uuid.NAMESPACE_DNS, "python.org"),
	// since our namespace constant is the standard DNS namespace from RFC 4122.
	want := "886313e1-3b8a-5372-9b90-0c9aee199e5d"
	got := GenerateUUID("python.org")
	if got != want {
		t.Errorf("GenerateUUID(%q) = %q, want %q", "python.org", got, want)
	}
}

func TestGenerateUUID_Deterministic(t *testing.T) {
	if GenerateUUID("apple") != GenerateUUID("apple") {
		t.Error("GenerateUUID is not deterministic for the same input")
	}
}

func TestGenerateUUID_DifferentInputsDiffer(t *testing.T) {
	if GenerateUUID("apple") == GenerateUUID("banana") {
		t.Error("GenerateUUID produced the same UUID for different inputs")
	}
}
