package store

import (
	"encoding/json"
	"os"
	"strings"
)

// ProviderSecretSettings stores optional secret indirections for DB providers.
type ProviderSecretSettings struct {
	APIKeyEnv string `json:"api_key_env,omitempty"`
}

// ParseProviderSecretSettings extracts provider secret settings from JSONB.
func ParseProviderSecretSettings(settings json.RawMessage) *ProviderSecretSettings {
	if len(settings) == 0 {
		return nil
	}
	var s ProviderSecretSettings
	if json.Unmarshal(settings, &s) != nil {
		return nil
	}
	s.APIKeyEnv = strings.TrimSpace(s.APIKeyEnv)
	if s.APIKeyEnv == "" {
		return nil
	}
	return &s
}

// ResolveProviderAPIKey returns the provider API key, preferring an env-backed
// override when settings.api_key_env is configured and present in the process env.
func ResolveProviderAPIKey(p *LLMProviderData) string {
	if p == nil {
		return ""
	}
	if secretSettings := ParseProviderSecretSettings(p.Settings); secretSettings != nil {
		if value := strings.TrimSpace(os.Getenv(secretSettings.APIKeyEnv)); value != "" {
			return value
		}
	}
	return strings.TrimSpace(p.APIKey)
}
