// Package catalog is the runtime translation engine for Africa OS.
package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"text/template"

	"github.com/combi/pkg/i18n/ctxlocale"
	"github.com/combi/pkg/i18n/locale"
	"github.com/combi/pkg/i18n/plurals"
)

// Entry is a single translation entry. It is either a simple string
// or a map of plural forms to strings.
type Entry struct {
	simple   string
	pluralM  map[plurals.Form]string
	isPlural bool
}

// UnmarshalJSON handles both string and plural-map JSON values.
func (e *Entry) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		// Simple string
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		e.simple = s
		return nil
	}
	// Plural map: {"one": "...", "other": "..."}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("catalog: entry must be string or plural map: %w", err)
	}
	e.pluralM = make(map[plurals.Form]string, len(raw))
	for k, v := range raw {
		e.pluralM[plurals.Form(k)] = v
	}
	e.isPlural = true
	return nil
}

// Resolve returns the correct string for the given count and locale.
// For simple entries, count is ignored.
func (e *Entry) Resolve(n int64, l locale.Locale) string {
	if !e.isPlural {
		return e.simple
	}
	form := plurals.Select(n, l)
	if s, ok := e.pluralM[form]; ok {
		return s
	}
	// Fallback: try "other"
	if s, ok := e.pluralM[plurals.FormOther]; ok {
		return s
	}
	return ""
}

// Catalog holds all translations for a single locale.
type Catalog struct {
	locale  locale.Locale
	entries map[string]*Entry
	// compiled holds pre-compiled templates for hot-path interpolation.
	compiled map[string]*template.Template
	mu       sync.RWMutex
}

// Registry manages catalogs for all loaded locales.
type Registry struct {
	catalogs map[locale.Locale]*Catalog
	mu       sync.RWMutex
}

// New creates an empty Registry.
func New() *Registry {
	return &Registry{
		catalogs: make(map[locale.Locale]*Catalog),
	}
}

// LoadFS loads all JSON catalog files from an fs.FS.
func (r *Registry) LoadFS(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("catalog: read %s: %w", path, err)
		}
		name := strings.TrimSuffix(d.Name(), ".json")
		l := locale.Locale(name)
		return r.LoadJSON(l, data)
	})
}

// LoadJSON parses and registers a JSON catalog for the given locale.
func (r *Registry) LoadJSON(l locale.Locale, data []byte) error {
	raw := make(map[string]*Entry)
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("catalog: parse %s: %w", string(l), err)
	}
	for key, entry := range raw {
		if entry != nil && entry.isPlural {
			if _, ok := entry.pluralM[plurals.FormOther]; !ok {
				return fmt.Errorf("catalog: %s: key %q: plural entry missing 'other' form", l, key)
			}
		}
	}
	c := &Catalog{
		locale:   l,
		entries:  raw,
		compiled: make(map[string]*template.Template, len(raw)),
	}
	r.mu.Lock()
	r.catalogs[l] = c
	r.mu.Unlock()
	return nil
}

// T is the primary translation function.
func (r *Registry) T(key string, l locale.Locale, data map[string]any, n int64) string {
	chain := locale.FallbackChain(l)
	for _, candidate := range chain {
		r.mu.RLock()
		c, ok := r.catalogs[candidate]
		r.mu.RUnlock()
		if !ok {
			continue
		}
		entry, ok := c.entries[key]
		if !ok {
			continue
		}
		raw := entry.Resolve(n, candidate)
		if raw != "" {
			return interpolate(c, key, raw, data)
		}
	}
	// Key not found in any fallback — return key itself (aids debugging)
	return key
}

// TPlural is a convenience wrapper for plural translations.
func (r *Registry) TPlural(key string, l locale.Locale, n int64, data map[string]any) string {
	return r.T(key, l, data, n)
}

// TCtx resolves the locale from ctx and calls T.
func (r *Registry) TCtx(ctx context.Context, key string, data map[string]any, n int64) string {
	l := ctxlocale.LocaleFromContext(ctx)
	return r.T(key, l, data, n)
}

// Has reports whether a key exists in any catalog in the fallback chain for l.
func (r *Registry) Has(key string, l locale.Locale) bool {
	for _, candidate := range locale.FallbackChain(l) {
		r.mu.RLock()
		c, ok := r.catalogs[candidate]
		r.mu.RUnlock()
		if !ok {
			continue
		}
		if _, ok := c.entries[key]; ok {
			return true
		}
	}
	return false
}

// Keys returns all translation keys loaded for the given locale.
func (r *Registry) Keys(l locale.Locale) []string {
	r.mu.RLock()
	c, ok := r.catalogs[l]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		keys = append(keys, k)
	}
	return keys
}

// ExportJSON returns the catalog for l as a JSON byte slice.
func (r *Registry) ExportJSON(l locale.Locale) ([]byte, error) {
	r.mu.RLock()
	c, ok := r.catalogs[l]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("catalog: locale %s not loaded", l)
	}
	out := make(map[string]any, len(c.entries))
	for key, entry := range c.entries {
		if entry == nil {
			continue
		}
		if !entry.isPlural {
			out[key] = entry.simple
			continue
		}
		m := make(map[string]string, len(entry.pluralM))
		for form, value := range entry.pluralM {
			m[string(form)] = value
		}
		out[key] = m
	}
	return json.Marshal(out)
}

// MissingKeys returns keys present in the reference locale but absent in l.
func (r *Registry) MissingKeys(reference, l locale.Locale) []string {
	refKeys := r.Keys(reference)
	var missing []string

	r.mu.RLock()
	targetCat, hasTarget := r.catalogs[l]
	r.mu.RUnlock()

	for _, k := range refKeys {
		if !hasTarget {
			missing = append(missing, k)
			continue
		}
		if _, ok := targetCat.entries[k]; !ok {
			missing = append(missing, k)
		}
	}
	return missing
}

func interpolate(c *Catalog, key, raw string, data map[string]any) string {
	if data == nil || !strings.Contains(raw, "{{") {
		return raw
	}

	cacheKey := key + "\x00" + raw

	c.mu.RLock()
	tmpl, ok := c.compiled[cacheKey]
	c.mu.RUnlock()

	if !ok {
		var err error
		tmpl, err = template.New(key).Parse(raw)
		if err != nil {
			return raw
		}
		c.mu.Lock()
		c.compiled[cacheKey] = tmpl
		c.mu.Unlock()
	}

	result, err := safeExecute(tmpl, data)
	if err != nil {
		return raw
	}
	return result
}

func safeExecute(tmpl *template.Template, data map[string]any) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("template panic: %v", r)
		}
	}()
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	return buf.String(), err
}
