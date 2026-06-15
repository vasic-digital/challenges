package bank

import (
	"fmt"
	"os"
)

// ValidationError represents a validation issue found in a bank file.
type ValidationError struct {
	Field   string
	Message string
	Index   int // -1 if not applicable
}

func (e ValidationError) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("challenges[%d].%s: %s", e.Index, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateFile validates a bank file structure and returns all errors found.
//
// It parses the file through parseBankFile — the SAME format-detecting
// parser LoadFile uses — so the validator accepts every file LoadFile
// accepts (JSON, .yaml, .yml) and reads the HelixQA `test_cases` root
// key (parseBankFile folds it into Challenges). A validator that
// rejected YAML, or that silently skipped test_cases validation, was a
// §11.4 / §11.4.1 validation-layer bluff: a malformed bank (duplicate
// IDs, missing names) would pass unnoticed.
func ValidateFile(path string) []ValidationError {
	var errors []ValidationError

	data, err := os.ReadFile(path)
	if err != nil {
		return []ValidationError{{Field: "file", Message: err.Error(), Index: -1}}
	}

	parsed, err := parseBankFile(path, data)
	if err != nil {
		// Field stays "json" for backward compatibility: it is the
		// stable historical label for a bank-file parse error
		// (callers may switch on it). The message carries the real
		// parser detail (JSON or YAML) so the cause is never hidden.
		return []ValidationError{{Field: "json", Message: err.Error(), Index: -1}}
	}
	file := *parsed

	if file.Version == "" {
		errors = append(errors, ValidationError{
			Field: "version", Message: "version is required", Index: -1,
		})
	}

	ids := make(map[string]bool)
	for i, ch := range file.Challenges {
		if ch.ID == "" {
			errors = append(errors, ValidationError{
				Field: "id", Message: "challenge ID is required", Index: i,
			})
		} else if ids[string(ch.ID)] {
			errors = append(errors, ValidationError{
				Field: "id", Message: fmt.Sprintf("duplicate ID: %s", ch.ID), Index: i,
			})
		} else {
			ids[string(ch.ID)] = true
		}

		if ch.Name == "" {
			errors = append(errors, ValidationError{
				Field: "name", Message: "challenge name is required", Index: i,
			})
		}
	}

	return errors
}
