package automation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"text/template"
)

// Global cache for parsed templates.
// Since we are in the internal service package, a singleton here is acceptable
// to avoid passing a Cache object to every single ActionHandler.
var templateCache = &paramTemplateCache{}

type paramTemplateCache struct {
	cache sync.Map // map[string]*template.Template
}

func (c *paramTemplateCache) GetOrParse(templateStr string) (*template.Template, error) {
	// Optimization: Empty strings or non-templates don't need parsing
	if templateStr == "" || !strings.Contains(templateStr, "{{") {
		return nil, nil
	}

	// 1. Generate Hash Key
	// SHA256 is fast enough, and collision resistant.
	// For extreme performance, FNV64a is faster but SHA is safer for arbitrary user input.
	hash := sha256.Sum256([]byte(templateStr))
	key := hex.EncodeToString(hash[:])

	// 2. Check Cache
	if val, ok := c.cache.Load(key); ok {
		return val.(*template.Template), nil
	}

	// 3. Parse (Slow Path)
	// "missingkey=error" is crucial for safety as discussed in Security section
	tmpl, err := template.New(key).Option("missingkey=error").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	// 4. Store
	c.cache.Store(key, tmpl)
	return tmpl, nil
}
