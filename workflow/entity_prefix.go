package workflow

import (
	"strings"
	"sync"
	"unicode"
)

// Default entity ID prefix segments.
const (
	DefaultOrg      = "semspec"
	DefaultPlatform = "local"
)

var (
	prefixMu sync.RWMutex
	prefix   = entityPrefixState{org: DefaultOrg, platform: DefaultPlatform}
)

type entityPrefixState struct {
	org      string
	platform string
}

// InitEntityPrefix sets the org and platform segments for entity IDs.
// Call once during startup from .semspec/project.json values.
//
// If platform is empty but projectName is provided, the platform is
// auto-derived as a slugified short form of the project name (e.g.,
// "My Cool Project" → "my-cool-project"). This gives each project a
// unique entity namespace without requiring explicit configuration.
//
// Empty strings are ignored (defaults preserved).
func InitEntityPrefix(org, platform, projectName string) {
	prefixMu.Lock()
	defer prefixMu.Unlock()
	if org != "" {
		prefix.org = org
	}
	if platform != "" {
		prefix.platform = platform
	} else if projectName != "" {
		prefix.platform = slugify(projectName)
	}
}

// EntityPrefix returns the "org.platform" prefix for entity IDs.
func EntityPrefix() string {
	prefixMu.RLock()
	defer prefixMu.RUnlock()
	return prefix.org + "." + prefix.platform
}

// slugify converts a project name to a short, dot-free slug suitable for
// use in a 6-part entity ID. Lowercases, replaces non-alphanumeric with
// hyphens, collapses runs, and trims.
func slugify(name string) string {
	var b strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = r
		} else if prev != '-' {
			b.WriteByte('-')
			prev = '-'
		}
	}
	return strings.Trim(b.String(), "-")
}
