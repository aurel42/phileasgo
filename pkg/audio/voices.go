package audio

// VoiceProfile represents a Gemini TTS voice.
type VoiceProfile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Style  string `json:"style"`
}

// GeminiVoices contains all available Gemini 2.5 TTS voice profiles.
var GeminiVoices = []VoiceProfile{
	{ID: "aoede", Name: "Aoede", Gender: "Female", Style: "Professional, capable, clear, experienced, composed"},
	{ID: "zephyr", Name: "Zephyr", Gender: "Female", Style: "Energetic, bright, youthful, fast-paced, spirited"},
	{ID: "kore", Name: "Kore", Gender: "Female", Style: "Calm, soothing, gentle, relaxed, soft"},
	{ID: "leda", Name: "Leda", Gender: "Female", Style: "Authoritative, formal, direct, sophisticated, commanding"},
	{ID: "algenib", Name: "Algenib", Gender: "Female", Style: "Warm, friendly, confident, engaging, approachable"},
	{ID: "callirrhoe", Name: "Callirrhoe", Gender: "Female", Style: "Expressive, quirky, versatile, distinctive, lively"},
	{ID: "charon", Name: "Charon", Gender: "Male", Style: "Deep, trustworthy, conversational, smooth, steady"},
	{ID: "fenrir", Name: "Fenrir", Gender: "Male", Style: "Resonant, intense, strong, gravelly, dramatic"},
	{ID: "puck", Name: "Puck", Gender: "Male", Style: "Playful, mischievous, energetic, animated, humorous"},
	{ID: "orus", Name: "Orus", Gender: "Male", Style: "Balanced, versatile, neutral, clear, reliable"},
	{ID: "umbriel", Name: "Umbriel", Gender: "Male", Style: "Authoritative, narrator-like, wise, grounded, engaging"},
	{ID: "sadachbia", Name: "Sadachbia", Gender: "Male", Style: "Laid-back, cool, textured, relaxed, casual"},
}

// SupportedLanguages lists languages supported by Gemini TTS.
var SupportedLanguages = []string{
	"English", "French", "German", "Spanish", "Italian",
	"Portuguese", "Russian", "Japanese", "Chinese", "Arabic",
}

// GetVoiceByID returns a voice profile by ID, or the first voice if not found.
func GetVoiceByID(id string) VoiceProfile {
	for _, v := range GeminiVoices {
		if v.ID == id {
			return v
		}
	}
	return GeminiVoices[0]
}
