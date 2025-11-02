package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
)

//go:embed translations/en.json
var enJSON []byte

//go:embed translations/zh.json
var zhJSON []byte

var (
	// currentLang represents the current language setting (default: zh)
	currentLang = "zh"

	// translations stores all loaded translations
	translations = make(map[string]map[string]string)
)

func init() {
	// Load English translations from embedded JSON
	var enTranslations map[string]string
	if err := json.Unmarshal(enJSON, &enTranslations); err != nil {
		log.Fatalf("Failed to load English translations: %v", err)
	}
	translations["en"] = enTranslations

	// Load Chinese translations from embedded JSON
	var zhTranslations map[string]string
	if err := json.Unmarshal(zhJSON, &zhTranslations); err != nil {
		log.Fatalf("Failed to load Chinese translations: %v", err)
	}
	translations["zh"] = zhTranslations
}

// SetLanguage sets the current language ("en" or "zh")
func SetLanguage(lang string) {
	if lang == "en" {
		currentLang = "en"
	} else {
		currentLang = "zh"
	}
}

// GetLanguage returns the current language
func GetLanguage() string {
	return currentLang
}

// T translates a key to the current language with optional formatting
// Usage:
//   - T("key") - simple translation
//   - T("key", arg1, arg2) - translation with fmt.Sprintf formatting
//
// Falls back to English if translation not found in current language,
// returns the key itself if not found in any language.
func T(key string, args ...any) string {
	// Try current language first
	msg, exists := translations[currentLang][key]
	if !exists {
		// Fallback to English
		msg, exists = translations["en"][key]
		if !exists {
			// Return key if translation not found in any language
			return key
		}
	}

	// Apply formatting if arguments provided
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}

	return msg
}
