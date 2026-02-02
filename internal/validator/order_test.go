package validator

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateOrderNumber_Success(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid number", "4561261212345467", "4561261212345467"},
		{"valid with spaces", "  4561261212345467  ", "4561261212345467"},
		{"valid with tabs", "\t4561261212345467\t", "4561261212345467"},
		{"valid single digit", "0", "0"},
		{"valid long number", "79927398713", "79927398713"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateOrderNumber(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidateOrderNumber_Empty(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"only spaces", "   "},
		{"only tabs", "\t\t"},
		{"mixed whitespace", " \t \n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateOrderNumber(tt.input)
			if !errors.Is(err, ErrEmptyOrderNumber) {
				t.Errorf("expected ErrEmptyOrderNumber, got %v", err)
			}
		})
	}
}

func TestValidateOrderNumber_InvalidCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"letters", "abc123"},
		{"special chars", "123-456"},
		{"with dot", "123.456"},
		{"with comma", "123,456"},
		{"with space in middle", "123 456"},
		{"alphanumeric", "A1B2C3"},
		{"unicode", "12345🔢"},
		{"cyrillic", "12345б"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateOrderNumber(tt.input)
			var errInvalidNumber *ErrInvalidOrderNumber
			if !errors.As(err, &errInvalidNumber) {
				t.Errorf("expected ErrInvalidOrderNumber, got %v", err)
			}
		})
	}
}

func TestValidateOrderNumber_InvalidLuhn(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid luhn 1", "1234567890"},
		{"invalid luhn 2", "1111111111"},
		{"invalid luhn 3", "12345"},
		{"invalid luhn 4", "4561261212345468"}, // off by one
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateOrderNumber(tt.input)
			var errInvalidLuhn *ErrInvalidLuhn
			if !errors.As(err, &errInvalidLuhn) {
				t.Errorf("expected ErrInvalidLuhn, got %v", err)
			}
		})
	}
}

func TestValidateLuhn_ValidNumbers(t *testing.T) {
	validNumbers := []string{
		"0",                 // single digit
		"4561261212345467",  // valid credit card
		"79927398713",       // valid number
		"49927398716",       // valid number
		"1234567897",        // valid number
		"4532015112830366",  // valid VISA
		"6011111111111117",  // valid Discover
		"5555555555554444",  // valid MasterCard
		"378282246310005",   // valid Amex
	}

	for _, number := range validNumbers {
		t.Run(number, func(t *testing.T) {
			if !ValidateLuhn(number) {
				t.Errorf("expected %s to be valid", number)
			}
		})
	}
}

func TestValidateLuhn_InvalidNumbers(t *testing.T) {
	invalidNumbers := []string{
		"1234567890",       // invalid
		"1111111111",       // invalid
		"4561261212345468", // off by one
		"12345",            // invalid
		"0000000001",       // invalid
		"1234567891",       // invalid
	}

	for _, number := range invalidNumbers {
		t.Run(number, func(t *testing.T) {
			if ValidateLuhn(number) {
				t.Errorf("expected %s to be invalid", number)
			}
		})
	}
}

func TestValidateLuhn_EmptyString(t *testing.T) {
	if !ValidateLuhn("") {
		t.Error("expected empty string to be valid (sum=0, divisible by 10)")
	}
}

func TestValidateLuhn_SingleDigits(t *testing.T) {
	tests := []struct {
		digit string
		valid bool
	}{
		{"0", true},
		{"1", false},
		{"2", false},
		{"3", false},
		{"4", false},
		{"5", false},
		{"6", false},
		{"7", false},
		{"8", false},
		{"9", false},
	}

	for _, tt := range tests {
		t.Run(tt.digit, func(t *testing.T) {
			result := ValidateLuhn(tt.digit)
			if result != tt.valid {
				t.Errorf("expected %s to be valid=%v, got %v", tt.digit, tt.valid, result)
			}
		})
	}
}

func TestValidateLuhn_NonDigitCharacters(t *testing.T) {
	invalidInputs := []string{
		"123a456",
		"12-34",
		"12.34",
		"12 34",
		"abc",
		"12e4",
	}

	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			if ValidateLuhn(input) {
				t.Errorf("expected %s to be invalid", input)
			}
		})
	}
}

func TestValidateLuhn_LongNumber(t *testing.T) {
	longNumber := "79927398713" + strings.Repeat("0", 100)
	checksum := calculateLuhnChecksum(longNumber[:len(longNumber)-1])
	checksumStr := string(rune('0' + checksum))
	longValidNumber := longNumber[:len(longNumber)-1] + checksumStr

	if !ValidateLuhn(longValidNumber) {
		t.Error("expected long valid number to pass")
	}
}

func TestValidateLuhn_Algorithm(t *testing.T) {
	tests := []struct {
		number   string
		expected bool
	}{
		{"79927398713", true},
		{"79927398710", false},
		{"79927398712", false},
	}

	for _, tt := range tests {
		t.Run(tt.number, func(t *testing.T) {
			result := ValidateLuhn(tt.number)
			if result != tt.expected {
				t.Errorf("number %s: expected %v, got %v", tt.number, tt.expected, result)
			}
		})
	}
}

func TestValidateOrderNumber_Integration(t *testing.T) {
	t.Run("valid with whitespace", func(t *testing.T) {
		result, err := ValidateOrderNumber("  4561261212345467  ")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("empty after trim", func(t *testing.T) {
		_, err := ValidateOrderNumber("   ")
		if !errors.Is(err, ErrEmptyOrderNumber) {
			t.Errorf("expected ErrEmptyOrderNumber, got %v", err)
		}
	})

	t.Run("contains letters", func(t *testing.T) {
		_, err := ValidateOrderNumber("456126121234546A")
		var errInvalidNumber *ErrInvalidOrderNumber
		if !errors.As(err, &errInvalidNumber) {
			t.Errorf("expected ErrInvalidOrderNumber, got %v", err)
		}
	})

	t.Run("invalid luhn", func(t *testing.T) {
		_, err := ValidateOrderNumber("4561261212345468")
		var errInvalidLuhn *ErrInvalidLuhn
		if !errors.As(err, &errInvalidLuhn) {
			t.Errorf("expected ErrInvalidLuhn, got %v", err)
		}
	})

	t.Run("valid zero", func(t *testing.T) {
		result, err := ValidateOrderNumber("0")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == "" {
			t.Error("expected non-empty result")
		}
	})
}

func TestValidateOrderNumber_Normalization(t *testing.T) {
	inputs := []string{
		" 4561261212345467 ",
		"4561261212345467",
		"\t4561261212345467\n",
		"  \t  4561261212345467  \n  ",
	}

	expected := "4561261212345467"

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			result, err := ValidateOrderNumber(input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != expected {
				t.Errorf("expected %s, got %s", expected, result)
			}
		})
	}
}

func calculateLuhnChecksum(number string) int {
	var sum int
	alternate := true

	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')

		if alternate {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		alternate = !alternate
	}

	return (10 - (sum % 10)) % 10
}

func TestValidateLuhn_DoubleZero(t *testing.T) {
	if !ValidateLuhn("00") {
		t.Error("expected 00 to be valid")
	}
}

func TestValidateLuhn_TenZeros(t *testing.T) {
	if !ValidateLuhn("0000000000") {
		t.Error("expected 0000000000 to be valid")
	}
}
