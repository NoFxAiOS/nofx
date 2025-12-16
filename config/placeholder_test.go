package config

import (
	"strings"
	"testing"
)

// Copy of the old implementation for benchmarking comparison
func oldConvertPlaceholders(query string) string {
	result := query
	index := 1
	for strings.Contains(result, "?") {
		result = strings.Replace(result, "?", "placeholder", 1) // Simplified for benchmark avoid fmt dependency if possible, but original used fmt
		index++
	}
	return result
}

func TestConvertPlaceholders(t *testing.T) {
	// We need to access the method on the struct, but it's attached to Database.
	// Since we can't easily instantiate a full Database connection without a real DB in this simple unit test,
	// and the method doesn't actually use the db field, we can use a zero-value struct if the method is pure.
	// However, checking the code, `convertPlaceholders` IS a method on `*Database`.
	// Let's check if it uses `d` fields. It does NOT. It's a pure function attached to the struct.

	db := &Database{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No placeholders",
			input:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "One placeholder",
			input:    "SELECT * FROM users WHERE id = ?",
			expected: "SELECT * FROM users WHERE id = $1",
		},
		{
			name:     "Multiple placeholders",
			input:    "INSERT INTO users (name, age, email) VALUES (?, ?, ?)",
			expected: "INSERT INTO users (name, age, email) VALUES ($1, $2, $3)",
		},
		{
			name:     "Placeholders with text around",
			input:    "SELECT ? AS a, ? AS b",
			expected: "SELECT $1 AS a, $2 AS b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := db.convertPlaceholders(tt.input); got != tt.expected {
				t.Errorf("convertPlaceholders() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func BenchmarkConvertPlaceholders_New(b *testing.B) {
	db := &Database{}
	query := "INSERT INTO large_table (col1, col2, col3, col4, col5) VALUES (?, ?, ?, ?, ?)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.convertPlaceholders(query)
	}
}

// To benchmark the old way, we simulate the logic here
func BenchmarkConvertPlaceholders_Old(b *testing.B) {
	query := "INSERT INTO large_table (col1, col2, col3, col4, col5) VALUES (?, ?, ?, ?, ?)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := query
		index := 1
		for strings.Contains(result, "?") {
			// We use simple string concat to simulate the fmt.Sprintf overhead slightly or just replace
			// The original used fmt.Sprintf("$%d", index)
			// We can't import fmt inside the loop for efficiency in the "old" code if it wasn't there,
			// but we want to match the original logic.
			// Original: result = strings.Replace(result, "?", fmt.Sprintf("$%d", index), 1)
			replacement := string(rune('0' + index)) // Simplified for dependency, roughly equivalent work
			result = strings.Replace(result, "?", "$"+replacement, 1)
			index++
		}
	}
}
