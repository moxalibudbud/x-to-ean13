package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DefaultCompanyPrefix is used by UUIDToEAN13 and GetOrGenerateEAN13 when no
// prefix is supplied.
const DefaultCompanyPrefix = "200"

var ean13Pattern = regexp.MustCompile(`^\d{13}$`)

// calculateEAN13CheckDigit computes the EAN-13 check digit for a 12-digit code.
func calculateEAN13CheckDigit(code string) (string, error) {
	if len(code) != 12 {
		return "", fmt.Errorf("code must be 12 digits for EAN-13 check digit calculation, got %d", len(code))
	}

	sum := 0
	for i := 0; i < 12; i++ {
		digit, err := strconv.Atoi(string(code[i]))
		if err != nil {
			return "", fmt.Errorf("invalid digit %q at position %d", code[i], i)
		}
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}

	checkDigit := (10 - (sum % 10)) % 10
	return strconv.Itoa(checkDigit), nil
}

// UUIDToEAN13 deterministically derives a 13-digit EAN-13 barcode from uuid.
// The same uuid always produces the same EAN-13. Pass "" for companyPrefix to
// use DefaultCompanyPrefix.
func UUIDToEAN13(uuid string, companyPrefix string) (string, error) {
	if companyPrefix == "" {
		companyPrefix = DefaultCompanyPrefix
	}

	hexString := strings.ReplaceAll(uuid, "-", "")
	if len(hexString) != 32 {
		return "", fmt.Errorf("invalid UUID format: %s", uuid)
	}

	last8Hex := hexString[len(hexString)-8:]
	decimalValue, err := strconv.ParseUint(last8Hex, 16, 64)
	if err != nil {
		return "", fmt.Errorf("invalid UUID format: %s", uuid)
	}

	sequence := decimalValue % 100000000
	paddedSequence := fmt.Sprintf("%08d", sequence)
	code12 := (companyPrefix + paddedSequence + "0")[:12]

	checkDigit, err := calculateEAN13CheckDigit(code12)
	if err != nil {
		return "", err
	}

	return code12 + checkDigit, nil
}

// ValidateEAN13 reports whether ean13 is 13 digits with a correct check digit.
func ValidateEAN13(ean13 string) bool {
	if !ean13Pattern.MatchString(ean13) {
		return false
	}

	code12 := ean13[:12]
	providedCheckDigit := ean13[12:]
	calculatedCheckDigit, err := calculateEAN13CheckDigit(code12)
	if err != nil {
		return false
	}

	return providedCheckDigit == calculatedCheckDigit
}

// GetOrGenerateEAN13 returns ean13 if it's already valid, otherwise derives
// one from uuid (typically a UUID).
func GetOrGenerateEAN13(ean13 *string, uuid string, companyPrefix string) (string, error) {
	if ean13 != nil && ValidateEAN13(*ean13) {
		return *ean13, nil
	}

	return UUIDToEAN13(uuid, companyPrefix)
}
