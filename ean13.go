package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DefaultCompanyPrefix is used when no prefix is supplied. 200 sits in the GS1
// "restricted distribution" range (020-029, 040-049, 200-299), which is
// reserved for use inside a single company. Codes built on it are NOT valid
// GTINs for open trade: they are not globally unique and must never be
// submitted to retailers, marketplaces, or GDSN.
const DefaultCompanyPrefix = "200"

// payloadLength is the number of EAN-13 digits before the check digit.
const payloadLength = 12

var (
	ean13Pattern = regexp.MustCompile(`^\d{13}$`)

	// A company prefix must leave at least one digit of item reference inside
	// the 12-digit payload, so it can never be longer than 11 digits.
	companyPrefixPattern = regexp.MustCompile(`^\d{1,11}$`)
)

// calculateEAN13CheckDigit computes the EAN-13 check digit for a 12-digit code.
func calculateEAN13CheckDigit(code string) (string, error) {
	if len(code) != payloadLength {
		return "", fmt.Errorf("code must be 12 digits for EAN-13 check digit calculation, got %d", len(code))
	}

	sum := 0
	for i := 0; i < payloadLength; i++ {
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

// ValidateCompanyPrefix reports whether prefix can be used to build EAN-13
// codes: 1 to 11 digits, so that at least one item reference digit remains
// within the 12-digit payload.
func ValidateCompanyPrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("company prefix must not be empty")
	}
	if !companyPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("company prefix must be 1-11 digits, got %q", prefix)
	}
	return nil
}

// ItemRefLen returns how many digits of item reference are available after
// prefix. Callers must have validated prefix first.
func ItemRefLen(prefix string) int {
	return payloadLength - len(prefix)
}

// ItemRefCapacity returns the number of distinct item references addressable
// after prefix, i.e. one past the largest usable item reference.
func ItemRefCapacity(prefix string) uint64 {
	capacity := uint64(1)
	for i := 0; i < ItemRefLen(prefix); i++ {
		capacity *= 10
	}
	return capacity
}

// BuildEAN13 assembles the EAN-13 for a company prefix and an item reference,
// zero-padding the item reference to fill the payload and appending the check
// digit. It returns an error rather than truncating when itemRef does not fit
// in the digits the prefix leaves available.
func BuildEAN13(prefix string, itemRef uint64) (string, error) {
	if err := ValidateCompanyPrefix(prefix); err != nil {
		return "", err
	}

	width := ItemRefLen(prefix)
	if capacity := ItemRefCapacity(prefix); itemRef >= capacity {
		return "", fmt.Errorf(
			"item reference %d does not fit in the %d digit(s) left by company prefix %q (max %d)",
			itemRef, width, prefix, capacity-1,
		)
	}

	code12 := prefix + fmt.Sprintf("%0*d", width, itemRef)
	checkDigit, err := calculateEAN13CheckDigit(code12)
	if err != nil {
		return "", err
	}

	return code12 + checkDigit, nil
}

// ItemRefOf extracts the item reference that BuildEAN13 encoded into ean13 for
// the given company prefix.
func ItemRefOf(prefix, ean13 string) (uint64, error) {
	if err := ValidateCompanyPrefix(prefix); err != nil {
		return 0, err
	}
	if !ean13Pattern.MatchString(ean13) {
		return 0, fmt.Errorf("not a 13-digit code: %q", ean13)
	}
	if !strings.HasPrefix(ean13, prefix) {
		return 0, fmt.Errorf("code %q does not start with company prefix %q", ean13, prefix)
	}

	return strconv.ParseUint(ean13[len(prefix):payloadLength], 10, 64)
}

// PrefixClass describes what a GS1 prefix (the leading three digits of an
// EAN-13) may legitimately be used for.
type PrefixClass int

const (
	// PrefixLicensable is a range GS1 allocates to member organisations and
	// licenses to companies. Minting codes here without a license means
	// colliding with somebody else's real GTINs.
	PrefixLicensable PrefixClass = iota

	// PrefixRestricted is reserved for restricted distribution: codes used
	// only inside one company. Not globally unique, not valid for open trade.
	PrefixRestricted

	// PrefixReserved is set aside for a special purpose (publications,
	// coupons, refund receipts) and must not be used for trade items.
	PrefixReserved
)

func (c PrefixClass) String() string {
	switch c {
	case PrefixRestricted:
		return "restricted distribution (internal use only)"
	case PrefixReserved:
		return "reserved for a special purpose"
	default:
		return "licensable GS1 range"
	}
}

// classifyGS1Prefix classifies a single three-digit GS1 prefix value.
func classifyGS1Prefix(p int) PrefixClass {
	switch {
	case p >= 20 && p <= 29, // 020-029
		p >= 40 && p <= 49,   // 040-049
		p >= 200 && p <= 299: // 200-299
		return PrefixRestricted
	case p >= 977 && p <= 984, // serial publications, ISBN, ISMN, refunds, coupons
		p >= 990 && p <= 999: // coupons
		return PrefixReserved
	default:
		return PrefixLicensable
	}
}

// ClassifyPrefix classifies the GS1 prefix of code, which may be a full EAN-13
// or a shorter company prefix. When code is shorter than three digits it stands
// for a range of GS1 prefixes; the range is classified as a whole, falling back
// to PrefixLicensable (the conservative answer) if it spans several classes.
func ClassifyPrefix(code string) PrefixClass {
	if code == "" {
		return PrefixLicensable
	}

	known := len(code)
	if known > 3 {
		known = 3
	}
	value, err := strconv.Atoi(code[:known])
	if err != nil {
		return PrefixLicensable
	}

	span := 1
	for i := known; i < 3; i++ {
		value *= 10
		span *= 10
	}

	class := classifyGS1Prefix(value)
	for p := value + 1; p < value+span; p++ {
		if classifyGS1Prefix(p) != class {
			return PrefixLicensable
		}
	}
	return class
}

// ValidateEAN13 reports whether ean13 is 13 digits with a correct check digit.
//
// This is a check digit test only, NOT a test of GTIN validity: it returns true
// for structurally meaningless codes such as "0000000000000" and for reserved
// and restricted-distribution ranges. Use ValidateGTIN13 to also rule those out.
func ValidateEAN13(ean13 string) bool {
	if !ean13Pattern.MatchString(ean13) {
		return false
	}

	code12 := ean13[:payloadLength]
	providedCheckDigit := ean13[payloadLength:]
	calculatedCheckDigit, err := calculateEAN13CheckDigit(code12)
	if err != nil {
		return false
	}

	return providedCheckDigit == calculatedCheckDigit
}

// ValidateGTIN13 checks ean13 for structural validity as a trade item number:
// 13 digits, a correct check digit, a non-empty payload, and a prefix that is
// not reserved for another purpose. It deliberately accepts restricted
// distribution prefixes, which are valid for internal use; call ClassifyPrefix
// if you need to tell the two apart.
func ValidateGTIN13(ean13 string) error {
	if !ean13Pattern.MatchString(ean13) {
		return fmt.Errorf("%q is not 13 digits", ean13)
	}
	if !ValidateEAN13(ean13) {
		return fmt.Errorf("%q has an incorrect check digit", ean13)
	}
	if strings.Trim(ean13, "0") == "" {
		return fmt.Errorf("%q has an all-zero payload", ean13)
	}
	if class := ClassifyPrefix(ean13); class == PrefixReserved {
		return fmt.Errorf("%q uses a GS1 prefix %s", ean13, class)
	}
	return nil
}

// GetOrGenerateEAN13 returns ean13 when one is supplied, after checking it, and
// otherwise allocates a fresh code for key from ledger. The boolean reports
// whether a new code was allocated. A supplied but invalid code is an error:
// it is never silently replaced, so that bad upstream data stays visible.
func GetOrGenerateEAN13(ean13 *string, key string, ledger *Ledger) (string, bool, error) {
	if ean13 != nil && *ean13 != "" {
		if err := ValidateGTIN13(*ean13); err != nil {
			return "", false, fmt.Errorf("supplied EAN-13 for %q is invalid: %w", key, err)
		}
		return *ean13, false, nil
	}

	return ledger.Allocate(key)
}
