package generator

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/logging"
	"github.com/google/uuid"
)

// CreateSiteRequest bootstraps a new niche site.
type CreateSiteRequest struct {
	Slug              string   `json:"slug"`
	Niche             string   `json:"niche"`
	Tagline           string   `json:"tagline"`
	Description       string   `json:"description"`
	Domain            string   `json:"domain"`
	SeedKeywords      []string `json:"seed_keywords"`
	AffiliatePrograms []string `json:"affiliate_programs"`
	AdNetwork         string   `json:"ad_network"`
}

// CreateSite inserts a new Site row and triggers a keyword-map generation.
func (e *Engine) CreateSite(ctx context.Context, req CreateSiteRequest) (*db.Site, error) {
	if req.Niche == "" {
		return nil, fmt.Errorf("niche required")
	}
	if req.Slug == "" {
		req.Slug = Slugify(req.Niche)
	}
	site := &db.Site{
		ID:               uuid.NewString(),
		Slug:             req.Slug,
		Domain:           req.Domain,
		Niche:            req.Niche,
		Tagline:          req.Tagline,
		Description:      req.Description,
		SeedKeywords:     req.SeedKeywords,
		AffiliateProgram: req.AffiliatePrograms,
		AdNetwork:        req.AdNetwork,
		Status:           "draft",
		HealthScore:      1.0,
	}
	if err := e.db.InsertSite(ctx, site); err != nil {
		return nil, fmt.Errorf("insert site: %w", err)
	}
	// Fire-and-forget keyword map generation; if Claude not configured, we just skip.
	go func() {
		bgctx, cancel := newBackground(30 * time.Minute)
		defer cancel()
		bgctx = logging.WithRequestID(bgctx, "site-create-"+site.ID)
		if _, err := e.GenerateKeywordMap(bgctx, KeywordMapRequest{
			SiteID:       site.ID,
			Niche:        site.Niche,
			SeedKeywords: site.SeedKeywords,
			TargetCount:  40,
		}); err != nil {
			e.log.Warn(bgctx, "site-create: keyword map failed", map[string]any{"err": err.Error(), "site_id": site.ID})
		} else {
			e.log.Info(bgctx, "site-create: keyword map generated", map[string]any{"site_id": site.ID})
		}
	}()
	return site, nil
}

// PublishSite renders all published articles + home/category pages to disk and
// returns the output directory. Cowork takes this dir and uploads to Cloudflare Pages.
func (e *Engine) PublishSite(ctx context.Context, siteID, outDir string) (string, int, error) {
	site, err := e.db.GetSiteByID(ctx, siteID)
	if err != nil {
		return "", 0, fmt.Errorf("site: %w", err)
	}
	articles, err := e.db.ListArticlesBySite(ctx, site.ID)
	if err != nil {
		return "", 0, fmt.Errorf("articles: %w", err)
	}
	// Only include non-draft.
	published := make([]*db.Article, 0, len(articles))
	for _, a := range articles {
		if a.Status == "draft" {
			continue
		}
		published = append(published, a)
	}
	sort.Slice(published, func(i, j int) bool {
		return articleDate(published[i]).After(articleDate(published[j]))
	})

	root := filepath.Join(outDir, site.Slug, "public")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", 0, err
	}

	// --- Static assets (CSS) ---
	cssPath := filepath.Join(root, "style.css")
	if err := os.WriteFile(cssPath, []byte(defaultCSS), 0o644); err != nil {
		return "", 0, err
	}

	// --- Articles ---
	count := 0
	for _, a := range published {
		dir := filepath.Join(root, a.Slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", 0, err
		}
		pageHTML, err := renderArticlePage(site, a)
		if err != nil {
			return "", 0, err
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(pageHTML), 0o644); err != nil {
			return "", 0, err
		}
		count++
	}

	// --- Home + legal + sitemap + robots + ads.txt ---
	homeHTML, err := renderHomePage(site, published)
	if err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(homeHTML), 0o644); err != nil {
		return "", 0, err
	}

	for slug, html := range legalPages(site) {
		dir := filepath.Join(root, slug)
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644)
	}

	_ = os.WriteFile(filepath.Join(root, "sitemap.xml"), []byte(buildSitemap(site, published)), 0o644)
	_ = os.WriteFile(filepath.Join(root, "robots.txt"), []byte(buildRobots(site)), 0o644)
	if site.AdsTxt != "" {
		_ = os.WriteFile(filepath.Join(root, "ads.txt"), []byte(site.AdsTxt), 0o644)
	}

	e.log.Info(ctx, "site: published", map[string]any{
		"site_id":    site.ID,
		"output_dir": root,
		"articles":   count,
	})
	return root, count, nil
}

func articleDate(a *db.Article) time.Time {
	if a.PublishedAt != nil {
		return *a.PublishedAt
	}
	return a.CreatedAt
}

// ---- Templates ----

var baseLayout = template.Must(template.New("layout").Funcs(template.FuncMap{
	"safeHTML": func(s string) template.HTML { return template.HTML(s) },
	"safeJS":   func(s string) template.JS { return template.JS(s) },
	"year":     func() int { return time.Now().Year() },
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.CanonicalURL}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.CanonicalURL}}">
<meta property="og:type" content="{{.OGType}}">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{{.Title}}">
<meta name="twitter:description" content="{{.Description}}">
<link rel="stylesheet" href="/style.css">
{{if .SchemaJSONLD}}<script type="application/ld+json">{{.SchemaJSONLD | safeJS}}</script>{{end}}
{{if .FAQJSONLD}}<script type="application/ld+json">{{.FAQJSONLD | safeJS}}</script>{{end}}
</head>
<body>
<header class="site-header">
  <div class="container">
    <a class="brand" href="/">{{.Site.Slug}}</a>
    <nav>
      <a href="/">Home</a>
      <a href="/about/">About</a>
      <a href="/contact/">Contact</a>
    </nav>
  </div>
</header>
<div class="ad-slot ad-header" data-slot="header"></div>
<main class="container">
{{.Body | safeHTML}}
</main>
<div class="ad-slot ad-footer" data-slot="footer"></div>
<footer class="site-footer">
  <div class="container">
    <p>© {{year}} {{.Site.Slug}}. {{.Site.Tagline}}</p>
    <nav>
      <a href="/about/">About</a>
      <a href="/contact/">Contact</a>
      <a href="/privacy/">Privacy</a>
      <a href="/terms/">Terms</a>
    </nav>
    <p class="disclosure">Affiliate disclosure: We earn commissions from qualifying purchases. Reviews reflect our honest opinions.</p>
  </div>
</footer>
<div id="cookie-banner" class="cookie-banner">
  <p>We use cookies for analytics and advertising. By using this site you agree to our <a href="/privacy/">privacy policy</a>.</p>
  <button onclick="this.parentElement.style.display='none'">Accept</button>
</div>
</body>
</html>`))

type layoutData struct {
	Site         *db.Site
	Title        string
	Description  string
	CanonicalURL string
	OGType       string
	Body         string
	SchemaJSONLD string
	FAQJSONLD    string
}

func renderArticlePage(site *db.Site, a *db.Article) (string, error) {
	pub := articleDate(a)
	updated := a.UpdatedAt
	// Breadcrumbs and Article schema.
	base := canonicalBase(site)
	artURL := base + "/" + a.Slug + "/"
	articleSchema := fmt.Sprintf(`{"@context":"https://schema.org","@type":"Article","headline":%q,"datePublished":%q,"dateModified":%q,"author":{"@type":"Organization","name":%q},"mainEntityOfPage":%q}`,
		a.Title, pub.Format(time.RFC3339), updated.Format(time.RFC3339), a.Author, artURL)

	// Replace affiliate placeholders with attributed <a> tags (nofollow sponsored).
	body := rewriteAffiliateLinks(a.BodyHTML, site.Slug, a.Slug)

	// Inject "last updated" chip and breadcrumb.
	var prelude strings.Builder
	prelude.WriteString(`<nav class="breadcrumbs"><a href="/">Home</a> › <span>` + templateEscape(a.Title) + `</span></nav>`)
	prelude.WriteString(fmt.Sprintf(`<p class="article-meta">By %s · Published %s · Updated %s</p>`, templateEscape(a.Author), pub.Format("Jan 2, 2006"), updated.Format("Jan 2, 2006")))
	fullBody := prelude.String() + `<article class="article">` + body + `</article>` + inlineAdSlot()

	var sb strings.Builder
	err := baseLayout.Execute(&sb, layoutData{
		Site:         site,
		Title:        a.Title,
		Description:  a.MetaDescription,
		CanonicalURL: artURL,
		OGType:       "article",
		Body:         fullBody,
		SchemaJSONLD: articleSchema,
		FAQJSONLD:    a.FAQJSONLD,
	})
	return sb.String(), err
}

func renderHomePage(site *db.Site, articles []*db.Article) (string, error) {
	base := canonicalBase(site)
	var body strings.Builder
	fmt.Fprintf(&body, `<section class="hero"><h1>%s</h1><p>%s</p></section>`, templateEscape(firstNonEmpty(site.Tagline, site.Niche)), templateEscape(firstNonEmpty(site.Description, "")))
	body.WriteString(`<section class="article-grid">`)
	for _, a := range articles {
		fmt.Fprintf(&body, `<a class="card" href="/%s/"><h2>%s</h2><p>%s</p></a>`, a.Slug, templateEscape(a.Title), templateEscape(a.MetaDescription))
	}
	body.WriteString(`</section>`)
	var sb strings.Builder
	err := baseLayout.Execute(&sb, layoutData{
		Site:         site,
		Title:        firstNonEmpty(site.Tagline, site.Niche),
		Description:  firstNonEmpty(site.Description, site.Niche),
		CanonicalURL: base + "/",
		OGType:       "website",
		Body:         body.String(),
		SchemaJSONLD: fmt.Sprintf(`{"@context":"https://schema.org","@type":"Organization","name":%q,"url":%q}`, site.Slug, base),
	})
	return sb.String(), err
}

func legalPages(site *db.Site) map[string]string {
	pages := map[string]string{
		"about":   renderSimple(site, "About", aboutBody(site)),
		"contact": renderSimple(site, "Contact", contactBody(site)),
		"privacy": renderSimple(site, "Privacy Policy", privacyBody(site)),
		"terms":   renderSimple(site, "Terms of Service", termsBody(site)),
	}
	return pages
}

func renderSimple(site *db.Site, title, bodyHTML string) string {
	var sb strings.Builder
	_ = baseLayout.Execute(&sb, layoutData{
		Site:         site,
		Title:        title + " — " + site.Slug,
		Description:  title + " for " + site.Slug,
		CanonicalURL: canonicalBase(site) + "/" + Slugify(title) + "/",
		OGType:       "website",
		Body:         `<article class="article"><h1>` + title + `</h1>` + bodyHTML + `</article>`,
	})
	return sb.String()
}

func canonicalBase(site *db.Site) string {
	if site.Domain != "" {
		return "https://" + strings.TrimPrefix(site.Domain, "https://")
	}
	return "https://" + site.Slug + ".local"
}

func buildSitemap(site *db.Site, articles []*db.Article) string {
	base := canonicalBase(site)
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	fmt.Fprintf(&sb, "<url><loc>%s/</loc><priority>1.0</priority></url>\n", base)
	for _, a := range articles {
		lastmod := a.UpdatedAt.Format("2006-01-02")
		fmt.Fprintf(&sb, "<url><loc>%s/%s/</loc><lastmod>%s</lastmod><priority>0.8</priority></url>\n", base, a.Slug, lastmod)
	}
	for _, slug := range []string{"about", "contact", "privacy", "terms"} {
		fmt.Fprintf(&sb, "<url><loc>%s/%s/</loc><priority>0.3</priority></url>\n", base, slug)
	}
	sb.WriteString("</urlset>\n")
	return sb.String()
}

func buildRobots(site *db.Site) string {
	return "User-agent: *\nAllow: /\nDisallow: /admin/\nSitemap: " + canonicalBase(site) + "/sitemap.xml\n"
}

func aboutBody(site *db.Site) string {
	return fmt.Sprintf(`<p>%s is an independent editorial project covering %s. Our team of writers and analysts produce in-depth, research-driven guides and reviews.</p>
<h2>Our editorial standards</h2>
<p>Every article is fact-checked and updated regularly. We accept affiliate commissions on some outbound links — these never influence our rankings or recommendations.</p>
<h2>Contact</h2>
<p>Editorial inquiries: <a href="/contact/">contact page</a>.</p>`, site.Slug, site.Niche)
}

func contactBody(site *db.Site) string {
	return fmt.Sprintf(`<p>For editorial inquiries, tips, corrections, or partnership requests, email <strong>editor@%s</strong>.</p>
<p>We typically respond within 2 business days.</p>`, firstNonEmpty(site.Domain, site.Slug+".com"))
}

func privacyBody(site *db.Site) string {
	return `<p>We respect your privacy. This page summarizes what data we collect and how we use it.</p>
<h2>Cookies</h2>
<p>We use cookies for anonymous analytics (Google Analytics) and advertising (Ezoic / Mediavine / Google AdSense depending on site configuration). You can opt out in your browser.</p>
<h2>Affiliate links</h2>
<p>Many outbound links on this site are affiliate links. If you click and make a purchase, we may receive a commission at no extra cost to you.</p>
<h2>Data retention</h2>
<p>We do not store personal information except what is voluntarily submitted via our contact form.</p>
<h2>Your rights (GDPR/CCPA)</h2>
<p>You may request deletion of any data we hold on you by emailing privacy@` + firstNonEmpty(site.Domain, site.Slug+".com") + `.</p>`
}

func termsBody(site *db.Site) string {
	return `<p>By using this website you agree to the following terms.</p>
<h2>Content</h2>
<p>Content is provided for informational purposes. It does not constitute professional (financial, legal, medical) advice.</p>
<h2>Affiliate relationships</h2>
<p>We earn commissions from qualifying purchases via outbound affiliate links.</p>
<h2>Liability</h2>
<p>We are not liable for any losses incurred from decisions made based on our content.</p>`
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func templateEscape(s string) string {
	return template.HTMLEscapeString(s)
}

func inlineAdSlot() string {
	return `<aside class="ad-slot ad-sidebar" data-slot="sidebar"></aside>`
}

// rewriteAffiliateLinks swaps "AFFILIATE:program:code" placeholders into real
// <a> tags with rel="nofollow sponsored" and UTM params attached.
func rewriteAffiliateLinks(body, siteSlug, articleSlug string) string {
	// Patterns look like: href="AFFILIATE:amazon:B08XYZ123"
	reHref := fmt.Sprintf(`(?i)href="AFFILIATE:([a-z0-9_-]+):([a-z0-9_-]+)"`)
	_ = reHref // (regex not compiled here; we do a simpler replace below)
	// We do a linear scan to avoid importing regexp twice in different files.
	const token = `href="AFFILIATE:`
	var out strings.Builder
	i := 0
	for i < len(body) {
		j := strings.Index(body[i:], token)
		if j < 0 {
			out.WriteString(body[i:])
			break
		}
		out.WriteString(body[i : i+j])
		// Find closing quote.
		k := strings.Index(body[i+j+len(token):], `"`)
		if k < 0 {
			out.WriteString(body[i+j:])
			break
		}
		inner := body[i+j+len(token) : i+j+len(token)+k]
		parts := strings.SplitN(inner, ":", 2)
		program := "generic"
		code := inner
		if len(parts) == 2 {
			program = parts[0]
			code = parts[1]
		}
		utm := fmt.Sprintf("utm_source=%s&utm_medium=affiliate&utm_campaign=%s", siteSlug, articleSlug)
		placeholder := fmt.Sprintf(`href="/go/%s/%s/?%s" rel="nofollow sponsored" data-affiliate="%s"`, program, code, utm, program)
		out.WriteString(placeholder)
		i = i + j + len(token) + k + 1
	}
	return out.String()
}

// defaultCSS is a minimal, fast, mobile-first stylesheet designed for Core Web Vitals.
const defaultCSS = `*{box-sizing:border-box;margin:0;padding:0}
html{font-size:17px;-webkit-text-size-adjust:100%}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;line-height:1.65;color:#1a1a1a;background:#fafafa}
.container{max-width:760px;margin:0 auto;padding:1.2rem}
.site-header{border-bottom:1px solid #e5e5e5;background:#fff}
.site-header .container{display:flex;justify-content:space-between;align-items:center;padding:1rem 1.2rem}
.site-header .brand{font-weight:700;font-size:1.15rem;color:#111;text-decoration:none;text-transform:capitalize}
.site-header nav a{margin-left:1rem;color:#555;text-decoration:none}
.site-header nav a:hover{color:#0057b7}
.hero{padding:2rem 0;text-align:center}
.hero h1{font-size:2rem;margin-bottom:0.5rem}
.hero p{color:#555;font-size:1.05rem}
.article-grid{display:grid;grid-template-columns:1fr;gap:1rem;margin:1.5rem 0}
@media(min-width:640px){.article-grid{grid-template-columns:1fr 1fr}}
.card{display:block;padding:1.1rem;border:1px solid #e5e5e5;border-radius:8px;background:#fff;text-decoration:none;color:#111;transition:border-color .15s}
.card:hover{border-color:#0057b7}
.card h2{font-size:1.05rem;margin-bottom:.3rem;color:#111}
.card p{color:#555;font-size:.92rem}
.breadcrumbs{font-size:.85rem;color:#777;margin-bottom:.4rem}
.breadcrumbs a{color:#0057b7;text-decoration:none}
.article-meta{color:#888;font-size:.85rem;margin-bottom:1.2rem}
.article h1{font-size:2rem;line-height:1.2;margin-bottom:1rem}
.article h2{font-size:1.5rem;margin:2rem 0 0.8rem;line-height:1.3}
.article h3{font-size:1.2rem;margin:1.5rem 0 .5rem}
.article p{margin-bottom:1rem}
.article ul,.article ol{margin:0 0 1rem 1.5rem}
.article li{margin-bottom:.3rem}
.article table{width:100%;border-collapse:collapse;margin:1rem 0}
.article th,.article td{padding:.5rem;border:1px solid #e5e5e5;text-align:left}
.article th{background:#f5f5f5}
.article a{color:#0057b7}
.article a[rel~="sponsored"]{color:#b7410e;font-weight:600}
.ad-slot{min-height:90px;margin:1.5rem auto;text-align:center;color:#bbb;font-size:.8rem;max-width:760px}
.ad-slot::before{content:"advertisement";display:block;text-transform:uppercase;letter-spacing:.05em;font-size:.7rem;color:#bbb;margin-bottom:.3rem}
.site-footer{background:#111;color:#bbb;padding:2rem 0;margin-top:3rem}
.site-footer nav a{color:#ddd;margin-right:1rem;text-decoration:none}
.site-footer .disclosure{font-size:.8rem;color:#888;margin-top:.7rem}
.cookie-banner{position:fixed;bottom:0;left:0;right:0;padding:1rem;background:#111;color:#fff;display:flex;justify-content:space-between;align-items:center;gap:1rem}
.cookie-banner p{font-size:.9rem;flex:1}
.cookie-banner a{color:#8ab4f8}
.cookie-banner button{background:#0057b7;color:#fff;border:0;padding:.5rem 1rem;border-radius:4px;cursor:pointer}
`
