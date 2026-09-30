// internal/services/notification/renderer.go
package notification

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	"strings"
	"sync"
	texttemplate "text/template"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/log"
)

var _ notification.TemplateRenderer = (*Renderer)(nil)

type parsedTemplate struct {
	text *texttemplate.Template
	html *htmltemplate.Template
}

type Renderer struct {
	repo            notification.Repository
	localizationSvc localization.Service
	cache           cache.Service
	logger          log.Logger
	parsedCache     *sync.Map
}

// Updated constructor to accept the new dependency.
func NewRenderer(repo notification.Repository, cache cache.Service, logger log.Logger, localizationSvc localization.Service) *Renderer {
	return &Renderer{
		repo:            repo,
		cache:           cache,
		logger:          logger.With(log.String("component", "template_renderer")),
		parsedCache:     &sync.Map{},
		localizationSvc: localizationSvc,
	}
}

// Render is the main entry point
func (r *Renderer) Render(
	ctx context.Context,
	templateID uuid.UUID,
	data map[string]any,
	locale string,
) (*notification.RenderedTemplate, error) {

	template, err := r.repo.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("could not get template: %w", err)
	}

	// 1. Normalize Locale
	locCode := localization.LocaleCode(locale)
	if locCode == "" {
		locCode = localization.LocaleEnUS
	}

	// 2. Determine Keys
	subjectContentKey := ""
	if template.Subject != nil {
		subjectContentKey = *template.Subject
	}
	bodyContentKey := template.Body // e.g. "auth.verify_email.body"

	// 3. Parse Keys
	subjectResource, subjectKey := parseLocalizationKey(subjectContentKey)
	bodyResource, bodyKey := parseLocalizationKey(bodyContentKey)

	// 4. Fetch Actual Content from Localization Service
	// NOTE: This calls your DB. If it fails, it returns the key as fallback.
	localizedSubject, err := r.localizationSvc.GetTranslation(ctx, locCode, subjectResource, subjectKey)
	if err != nil {
		r.logger.Error("Error fetching subject translation", err)
		localizedSubject = subjectContentKey // Fallback to raw key
	}

	localizedBody, err := r.localizationSvc.GetTranslation(ctx, locCode, bodyResource, bodyKey)
	if err != nil {
		r.logger.Error("Error fetching body translation", err)
		localizedBody = bodyContentKey // Fallback to raw key
	}

	// 5. Execute Templates (Injecting Data)
	// We pass the locale here so caching is specific to the language
	renderedSubject, err := r.executeTemplate(template.ID, "subject", localizedSubject, data, string(locCode))
	if err != nil {
		return nil, fmt.Errorf("failed to render subject: %w", err)
	}

	renderedBody, err := r.executeTemplate(template.ID, "body", localizedBody, data, string(locCode))
	if err != nil {
		return nil, fmt.Errorf("failed to render body: %w", err)
	}

	return &notification.RenderedTemplate{
		Subject: &renderedSubject,
		Body:    renderedBody,
	}, nil
}

// executeTemplate parses and executes the template string
func (r *Renderer) executeTemplate(templateID uuid.UUID, part, tpl string, data map[string]any, locale string) (string, error) {
	// FIX: Cache key now includes locale to prevent EN/DE collisions
	cacheKey := fmt.Sprintf("%s-%s-%s", templateID.String(), part, locale)

	var tplWrapper *parsedTemplate
	cached, ok := r.parsedCache.Load(cacheKey)

	if ok {
		tplWrapper = cached.(*parsedTemplate)
	} else {
		// CRITICAL FIX: Do NOT cache if the content looks like a raw key (contains no spaces, has dots)
		// This prevents the system from getting stuck serving "auth.verify.body" if the DB was busy once.
		isRawKey := !strings.Contains(tpl, " ") && strings.Contains(tpl, ".") && len(tpl) < 80

		var err error
		tplWrapper = &parsedTemplate{}

		if part == "body" {
			// HTML Body
			tplWrapper.html, err = htmltemplate.New(cacheKey).Parse(tpl)
		} else {
			// Text Subject
			tplWrapper.text, err = texttemplate.New(cacheKey).Parse(tpl)
		}

		if err != nil {
			return "", err
		}

		// Only store in RAM cache if it looks like actual content (HTML/Text)
		if !isRawKey {
			r.parsedCache.Store(cacheKey, tplWrapper)
		} else {
			r.logger.Warn("Refusing to cache content that looks like a raw key", log.String("content", tpl))
		}
	}

	var buf bytes.Buffer
	var execErr error
	if tplWrapper.html != nil {
		execErr = tplWrapper.html.Execute(&buf, data)
	} else if tplWrapper.text != nil {
		execErr = tplWrapper.text.Execute(&buf, data)
	}

	if execErr != nil {
		return "", execErr
	}
	return buf.String(), nil
}

func parseLocalizationKey(fullKey string) (resource, key string) {
	if fullKey == "" {
		return "common", "missing"
	}
	parts := strings.SplitN(fullKey, ".", 2)
	if len(parts) == 1 {
		return "common", parts[0]
	}
	return parts[0], parts[1]
}
