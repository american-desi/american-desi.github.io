// Package generator turns a niche + keyword into a complete SEO-optimized article.
package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/american-desi/platform/internal/claude"
	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/logging"
	"github.com/google/uuid"
)

// Engine drives article + site generation.
type Engine struct {
	claude *claude.Client
	db     *db.DB
	log    *logging.Logger
}

// New returns an Engine.
func NewEngine(c *claude.Client, database *db.DB, log *logging.Logger) *Engine {
	return &Engine{claude: c, db: database, log: log}
}

// GenerateArticleRequest is the input contract.
type GenerateArticleRequest struct {
	SiteID            string   `json:"site_id"`
	FocusKeyword      string   `json:"focus_keyword"`
	SecondaryKeywords []string `json:"secondary_keywords,omitempty"`
	Cluster           string   `json:"cluster,omitempty"` // pillar | supporting | review | comparison
	ArticleType       string   `json:"article_type,omitempty"` // article | review | comparison | pillar
	TargetWordCount   int      `json:"target_word_count,omitempty"` // default 2000
	AffiliatePrograms []string `json:"affiliate_programs,omitempty"`
}

type generatedPayload struct {
	Title            string   `json:"title"`
	MetaDescription  string   `json:"meta_description"`
	Slug             string   `json:"slug"`
	BodyHTML         string   `json:"body_html"`
	SecondaryKeywords []string `json:"secondary_keywords"`
	FAQ              []struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	} `json:"faq"`
	AffiliateLinks []struct {
		Anchor         string `json:"anchor"`
		PlaceholderURL string `json:"placeholder_url"`
		Program        string `json:"program"`
		Position       string `json:"position"`
	} `json:"affiliate_links"`
	InternalLinkAnchors []string `json:"internal_link_anchors"`
	Verdict             string   `json:"verdict"`
	WordCount           int      `json:"word_count"`
}

// GenerateArticle creates a single article. Idempotent on (site, focus keyword, cluster).
func (e *Engine) GenerateArticle(ctx context.Context, req GenerateArticleRequest) (*db.Article, error) {
	if req.SiteID == "" || req.FocusKeyword == "" {
		return nil, fmt.Errorf("site_id and focus_keyword required")
	}
	if req.TargetWordCount == 0 {
		req.TargetWordCount = 2000
	}
	if req.ArticleType == "" {
		req.ArticleType = "article"
	}
	site, err := e.db.GetSiteByID(ctx, req.SiteID)
	if err != nil {
		return nil, fmt.Errorf("site lookup: %w", err)
	}

	// Idempotency: same hash returns the existing article.
	hash := Hash(site.ID, req.FocusKeyword, req.Cluster, req.ArticleType, fmt.Sprintf("%d", req.TargetWordCount))
	if existing, err := e.db.GetArticleByHash(ctx, hash); err == nil && existing != nil {
		e.log.Info(ctx, "generator: idempotent hit", map[string]any{
			"article_id": existing.ID,
			"hash":       hash,
		})
		return existing, nil
	}

	prompt := e.buildArticlePrompt(site, req)
	payload, resp, err := e.callArticleAPI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Post-process: enforce meta limits, sanitize HTML, add internal links, emit FAQ JSON-LD.
	title := truncateRunes(strings.TrimSpace(payload.Title), 60)
	desc := truncateRunes(strings.TrimSpace(payload.MetaDescription), 155)
	slug := payload.Slug
	if slug == "" {
		slug = Slugify(title)
	}
	slug = Slugify(slug)

	bodyHTML := sanitizeHTML(payload.BodyHTML)
	bodyHTML = e.injectInternalLinks(ctx, site.ID, bodyHTML, payload.InternalLinkAnchors)
	faqLD := buildFAQJSONLD(payload.FAQ)
	wordCount := wordCount(bodyHTML)
	if wordCount == 0 {
		wordCount = payload.WordCount
	}

	// Affiliate link objects.
	affLinks := make([]db.AffiliateLink, 0, len(payload.AffiliateLinks))
	for _, l := range payload.AffiliateLinks {
		affLinks = append(affLinks, db.AffiliateLink{
			Anchor:         l.Anchor,
			PlaceholderURL: l.PlaceholderURL,
			Program:        l.Program,
			Position:       l.Position,
			UTM:            fmt.Sprintf("utm_source=%s&utm_medium=affiliate&utm_campaign=%s", site.Slug, slug),
		})
	}

	var cost float64
	if resp != nil {
		cost = claude.CostUSD(resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}

	id := uuid.NewString()
	art := &db.Article{
		ID:                id,
		SiteID:            site.ID,
		Slug:              slug,
		Cluster:           req.Cluster,
		Type:              req.ArticleType,
		Title:             title,
		MetaDescription:   desc,
		FocusKeyword:      req.FocusKeyword,
		SecondaryKeywords: mergeStrings(req.SecondaryKeywords, payload.SecondaryKeywords),
		BodyHTML:          bodyHTML,
		FAQJSONLD:         faqLD,
		Author:            "Editorial Team",
		WordCount:         wordCount,
		Status:            "draft",
		GenerationCostUSD: cost,
		GenerationHash:    hash,
		AffiliateLinks:    affLinks,
	}
	if err := e.db.UpsertArticle(ctx, art); err != nil {
		return nil, fmt.Errorf("persist article: %w", err)
	}
	// Mark the keyword covered (best-effort).
	_ = e.db.MarkKeywordCovered(ctx, site.ID, req.FocusKeyword)

	e.log.Info(ctx, "generator: article created", map[string]any{
		"article_id":   id,
		"site_id":      site.ID,
		"focus":        req.FocusKeyword,
		"word_count":   wordCount,
		"cost_usd":     cost,
		"type":         req.ArticleType,
	})
	return art, nil
}

func (e *Engine) buildArticlePrompt(site *db.Site, req GenerateArticleRequest) string {
	var sb strings.Builder
	sb.WriteString("Generate a complete SEO article.\n\n")
	fmt.Fprintf(&sb, "Site niche: %s\n", site.Niche)
	if site.Tagline != "" {
		fmt.Fprintf(&sb, "Site tagline: %s\n", site.Tagline)
	}
	fmt.Fprintf(&sb, "Focus keyword: %s\n", req.FocusKeyword)
	fmt.Fprintf(&sb, "Article type: %s\n", req.ArticleType)
	if req.Cluster != "" {
		fmt.Fprintf(&sb, "Cluster role: %s\n", req.Cluster)
	}
	if len(req.SecondaryKeywords) > 0 {
		fmt.Fprintf(&sb, "Secondary keywords: %s\n", strings.Join(req.SecondaryKeywords, ", "))
	}
	if len(req.AffiliatePrograms) > 0 {
		fmt.Fprintf(&sb, "Affiliate programs available: %s\n", strings.Join(req.AffiliatePrograms, ", "))
	}
	fmt.Fprintf(&sb, "Target word count: %d (1500-3000 acceptable)\n", req.TargetWordCount)
	sb.WriteString(`
Requirements:
- Title ≤ 60 characters, contains focus keyword, click-worthy
- Meta description ≤ 155 characters, contains focus keyword, active voice
- URL slug: short, hyphenated, keyword-forward
- 1500-3000 words of genuinely helpful content
- H2/H3 structure. Each H2 answers a distinct search query.
- Include at least one definition paragraph (for featured snippet)
- Include at least one numbered or bulleted list
- For review/comparison articles: include an HTML <table> comparing options
- Natural keyword density 1-2% (do not stuff)
- 3-6 affiliate product mentions where relevant. Use placeholder URLs like "AFFILIATE:amazon:ASIN" — the deploy layer replaces them.
- 2-4 internal link anchors that reference related topics in the niche (exact article URLs will be injected later)
- 5-8 FAQ entries at the bottom (will become schema markup)
- End with a "Verdict" or "Our Recommendation" paragraph for commercial content
- HTML only, no markdown. No <html>/<body> wrappers. Start with <h1>.

Output a single JSON object with these keys:
{
  "title": "...",
  "meta_description": "...",
  "slug": "...",
  "body_html": "<h1>...</h1>...<h2>...</h2>...",
  "secondary_keywords": ["..."],
  "faq": [{"question":"...","answer":"..."}],
  "affiliate_links": [{"anchor":"...","placeholder_url":"AFFILIATE:program:code","program":"amazon","position":"in_content"}],
  "internal_link_anchors": ["related topic phrase", "another related topic"],
  "verdict": "...",
  "word_count": 2000
}`)
	return sb.String()
}

func (e *Engine) callArticleAPI(ctx context.Context, prompt string) (*generatedPayload, *claude.Response, error) {
	var payload generatedPayload
	resp, err := e.claude.CompleteJSON(ctx, prompt, claude.CompleteOpts{
		Purpose:     "article_gen",
		System:      ArticleSystemPrompt,
		Temperature: 0.7,
		MaxTokens:   8000,
	}, &payload)
	if err != nil {
		return nil, nil, fmt.Errorf("claude: %w", err)
	}
	if strings.TrimSpace(payload.BodyHTML) == "" {
		return nil, nil, fmt.Errorf("claude returned empty body_html")
	}
	return &payload, resp, nil
}

// injectInternalLinks links the first occurrence of each anchor phrase to a
// matching article on the same site, if one exists. Falls back to leaving text as-is.
func (e *Engine) injectInternalLinks(ctx context.Context, siteID, body string, anchors []string) string {
	if len(anchors) == 0 {
		return body
	}
	articles, err := e.db.ListArticlesBySite(ctx, siteID)
	if err != nil || len(articles) == 0 {
		return body
	}
	for _, anchor := range anchors {
		anchor = strings.TrimSpace(anchor)
		if anchor == "" {
			continue
		}
		best := matchArticle(anchor, articles)
		if best == nil {
			continue
		}
		// Replace first case-insensitive occurrence of anchor text outside existing <a> tags.
		body = replaceFirstOutsideAnchors(body, anchor, fmt.Sprintf(`<a href="/%s/">%s</a>`, best.Slug, html.EscapeString(anchor)))
	}
	return body
}

func matchArticle(anchor string, articles []*db.Article) *db.Article {
	lower := strings.ToLower(anchor)
	for _, a := range articles {
		if strings.Contains(strings.ToLower(a.Title), lower) || strings.Contains(strings.ToLower(a.FocusKeyword), lower) {
			return a
		}
	}
	return nil
}

// replaceFirstOutsideAnchors replaces the first case-insensitive match of needle
// with repl, avoiding text already inside <a ...>...</a> tags.
func replaceFirstOutsideAnchors(haystack, needle, repl string) string {
	// Split into segments at <a> boundaries; only replace in non-anchor segments.
	re := regexp.MustCompile(`(?is)<a\b[^>]*>.*?</a>`)
	idxPairs := re.FindAllStringIndex(haystack, -1)
	if len(idxPairs) == 0 {
		return replaceFirstCI(haystack, needle, repl)
	}
	var out strings.Builder
	prev := 0
	replaced := false
	for _, p := range idxPairs {
		chunk := haystack[prev:p[0]]
		if !replaced {
			newChunk, did := replaceFirstCIReport(chunk, needle, repl)
			if did {
				replaced = true
			}
			out.WriteString(newChunk)
		} else {
			out.WriteString(chunk)
		}
		out.WriteString(haystack[p[0]:p[1]])
		prev = p[1]
	}
	rest := haystack[prev:]
	if !replaced {
		rest, _ = replaceFirstCIReport(rest, needle, repl)
	}
	out.WriteString(rest)
	return out.String()
}

func replaceFirstCI(s, needle, repl string) string {
	out, _ := replaceFirstCIReport(s, needle, repl)
	return out
}

func replaceFirstCIReport(s, needle, repl string) (string, bool) {
	if needle == "" {
		return s, false
	}
	lower := strings.ToLower(s)
	idx := strings.Index(lower, strings.ToLower(needle))
	if idx < 0 {
		return s, false
	}
	return s[:idx] + repl + s[idx+len(needle):], true
}

// buildFAQJSONLD produces FAQPage schema markup.
func buildFAQJSONLD(faqs []struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}) string {
	if len(faqs) == 0 {
		return ""
	}
	type qa struct {
		Type     string `json:"@type"`
		Name     string `json:"name"`
		AcceptedAnswer struct {
			Type string `json:"@type"`
			Text string `json:"text"`
		} `json:"acceptedAnswer"`
	}
	type doc struct {
		Context    string `json:"@context"`
		Type       string `json:"@type"`
		MainEntity []qa   `json:"mainEntity"`
	}
	d := doc{Context: "https://schema.org", Type: "FAQPage"}
	for _, f := range faqs {
		var entry qa
		entry.Type = "Question"
		entry.Name = f.Question
		entry.AcceptedAnswer.Type = "Answer"
		entry.AcceptedAnswer.Text = f.Answer
		d.MainEntity = append(d.MainEntity, entry)
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func wordCount(body string) int {
	text := htmlTagRe.ReplaceAllString(body, " ")
	fields := strings.Fields(text)
	return len(fields)
}

// sanitizeHTML strips <script>, <iframe>, on* attrs and javascript: URLs.
func sanitizeHTML(s string) string {
	s = regexp.MustCompile(`(?is)<script.*?</script>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<iframe.*?</iframe>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\son\w+="[^"]*"`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\son\w+='[^']*'`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)javascript:`).ReplaceAllString(s, "#")
	return s
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func mergeStrings(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(a, b...) {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out
}

// Deprecated alias (kept to avoid import-cycle refactors).
var _ = time.Second
