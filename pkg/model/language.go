package model

// LanguageInfo holds the code and English name of a language.
type LanguageInfo struct {
	Code string `json:"code"` // e.g., "de"
	Name string `json:"name"` // e.g., "German"
}
