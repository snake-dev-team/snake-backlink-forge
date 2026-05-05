// Package templates — Renderer with sync.Map cache + fallback chain.
//
// Lookup order: bundles[lang][key] → bundles["vi"][key] → "[<key>]" literal.
// Parse errors → "[parse_error:<key>]" + zap.Error log.
// Execute errors → "[exec_error]" + zap.Error log.
//
// Concurrency: sync.Map is the only mutable state. text/template instances are
// safe for concurrent Execute after Parse, so cached *template.Template is
// shared across goroutines without further locking.
package templates

import (
	"strings"
	"sync"
	"text/template"

	"go.uber.org/zap"
)

// Renderer renders message templates by (lang, key) with caching.
type Renderer struct {
	log   *zap.Logger
	cache sync.Map // key: "<lang>/<tmplKey>" → *template.Template
}

// NewRenderer returns a Renderer that logs parse/exec failures via log.
// log MUST be non-nil; pass zap.NewNop() in tests if logging is undesired.
func NewRenderer(log *zap.Logger) *Renderer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Renderer{log: log}
}

// Render returns the rendered template for (lang, key) with data interpolation.
// On any failure, returns a bracket-literal sentinel and logs the cause.
//
// Fallback chain:
//   1. cache hit at "<lang>/<key>" → execute cached template
//   2. cache miss → look up bundles[lang][key]
//   3. lang missing key → fall back to bundles["vi"][key]
//   4. vi also missing → log warn + return "[<key>]"
//   5. parse error → log error + return "[parse_error:<key>]"
//   6. exec error → log error + return "[exec_error]"
func (r *Renderer) Render(lang, key string, data any) string {
	cacheKey := lang + "/" + key
	if v, ok := r.cache.Load(cacheKey); ok {
		return execTemplate(r.log, v.(*template.Template), data)
	}

	src, ok := lookupTemplate(lang, key)
	if !ok {
		r.log.Warn("templates: missing key in all bundles",
			zap.String("lang", lang), zap.String("key", key))
		return "[" + key + "]"
	}

	t, err := template.New(key).Parse(src)
	if err != nil {
		r.log.Error("templates: parse failed",
			zap.String("lang", lang), zap.String("key", key), zap.Error(err))
		return "[parse_error:" + key + "]"
	}

	r.cache.Store(cacheKey, t)
	return execTemplate(r.log, t, data)
}

// lookupTemplate returns the template source for (lang, key) with VN fallback.
// Second return is false only when neither lang nor "vi" has the key.
func lookupTemplate(lang, key string) (string, bool) {
	if b, ok := bundles[lang]; ok {
		if src, ok := b[key]; ok {
			return src, true
		}
	}
	// Fallback to Vietnamese (default).
	if lang != "vi" {
		if src, ok := bundles["vi"][key]; ok {
			return src, true
		}
	}
	return "", false
}

// execTemplate runs t.Execute against data and returns the rendered string.
// Errors are logged and "[exec_error]" is returned so callers always get
// a non-empty string suitable for sending to Telegram.
func execTemplate(log *zap.Logger, t *template.Template, data any) string {
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		log.Error("templates: execute failed",
			zap.String("name", t.Name()), zap.Error(err))
		return "[exec_error]"
	}
	return sb.String()
}
