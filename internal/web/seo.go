package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

const (
	metaDescriptionMax = 160
	rssPostLimit       = 30
	sitemapPostLimit   = 500
)

var imgSrcRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)

type SEOView struct {
	Title            string
	Description      string
	CanonicalURL     string
	Robots           string
	OGType           string
	OGTitle          string
	OGDescription    string
	OGURL            string
	OGImage          string
	OGLocale         string
	SiteName         string
	TwitterCard      string
	ArticlePublished string
	ArticleModified  string
	Author           string
	JSONLD           template.JS
	RSSURL           string
	RSSTitle         string
}

func noindexSEO() *SEOView {
	return &SEOView{Robots: "noindex, nofollow"}
}

func ogLocale(lang string) string {
	switch lang {
	case "fa":
		return "fa_IR"
	default:
		return "en_US"
	}
}

func metaDescription(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if utf8.RuneCountInString(text) <= metaDescriptionMax {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:metaDescriptionMax-1])) + "…"
}

func firstImageSrc(html string) string {
	match := imgSrcRe.FindStringSubmatch(html)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func absolutePublicURL(r *http.Request, baseDomain string, owner store.User, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	} else if path[0] != '/' {
		path = "/" + path
	}
	return PublicBlogURLForOwner(r, baseDomain, owner, path)
}

func absoluteAssetURL(r *http.Request, baseDomain string, owner store.User, src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return src
	}
	return absolutePublicURL(r, baseDomain, owner, src)
}

func defaultOGImageURL(r *http.Request, baseDomain string) string {
	return strings.TrimSuffix(appBaseURL(r, baseDomain), "/") + "/static/favicon.png"
}

func postDocumentTitle(title, blogTitle string) string {
	title = strings.TrimSpace(title)
	blogTitle = strings.TrimSpace(blogTitle)
	if title == "" {
		return blogTitle
	}
	if blogTitle == "" {
		return title
	}
	return blogTitle + " — " + title
}

func blogDescription(lang, displayName, bio string) string {
	if desc := metaDescription(bio); desc != "" {
		return desc
	}
	return i18n.T(lang, "seo.blog.description", displayName)
}

func postSEO(r *http.Request, baseDomain string, owner store.User, post store.Post) *SEOView {
	displayName := publicDisplayName(owner)
	authorName := publicAuthorName(owner)
	pageLang := blogPageLang(owner)
	lang := postLang(pageLang)
	description := metaDescription(postExcerpt(post.HTML, metaDescriptionMax))
	if description == "" {
		description = i18n.T(pageLang, "seo.post.description", post.Title, displayName)
	}

	canonical := absolutePublicURL(r, baseDomain, owner, "/"+post.Slug)
	blogTitle := i18n.T(pageLang, "profile.title", displayName)
	docTitle := postDocumentTitle(post.Title, blogTitle)
	image := absoluteAssetURL(r, baseDomain, owner, firstImageSrc(post.HTML))
	if image == "" {
		image = defaultOGImageURL(r, baseDomain)
	}

	seo := &SEOView{
		Title:         docTitle,
		Description:   description,
		CanonicalURL:  canonical,
		Robots:        "index, follow",
		OGType:        "article",
		OGTitle:       docTitle,
		OGDescription: description,
		OGURL:         canonical,
		OGImage:       image,
		OGLocale:      ogLocale(lang),
		SiteName:      displayName,
		TwitterCard:   "summary_large_image",
		Author:        authorName,
		RSSURL:        absolutePublicURL(r, baseDomain, owner, "/feed.xml"),
		RSSTitle:      displayName,
	}
	if post.PublishedAt != nil {
		seo.ArticlePublished = post.PublishedAt.UTC().Format(time.RFC3339)
	}
	seo.ArticleModified = post.UpdatedAt.UTC().Format(time.RFC3339)
	seo.JSONLD = postJSONLD(seo, post, lang)
	return seo
}

func blogSEO(r *http.Request, baseDomain string, owner store.User) *SEOView {
	displayName := publicDisplayName(owner)
	authorName := publicAuthorName(owner)
	lang := blogPageLang(owner)
	description := blogDescription(lang, displayName, owner.Bio)
	canonical := absolutePublicURL(r, baseDomain, owner, "/")
	ogImage := defaultOGImageURL(r, baseDomain)

	return &SEOView{
		Title:         i18n.T(lang, "profile.title", displayName),
		Description:   description,
		CanonicalURL:  canonical,
		Robots:        "index, follow",
		OGType:        "website",
		OGTitle:       displayName,
		OGDescription: description,
		OGURL:         canonical,
		OGImage:       ogImage,
		OGLocale:      ogLocale(lang),
		SiteName:      displayName,
		TwitterCard:   "summary_large_image",
		Author:        authorName,
		RSSURL:        absolutePublicURL(r, baseDomain, owner, "/feed.xml"),
		RSSTitle:      displayName,
		JSONLD:        blogJSONLD(displayName, description, canonical, authorName, lang, ogImage),
	}
}

func landingSEO(r *http.Request, baseDomain, lang string) *SEOView {
	canonical := strings.TrimSuffix(appBaseURL(r, baseDomain), "/") + "/"
	description := i18n.T(lang, "seo.landing.description")
	title := i18n.T(lang, "seo.landing.title")
	ogImage := defaultOGImageURL(r, baseDomain)
	return &SEOView{
		Title:         title,
		Description:   description,
		CanonicalURL:  canonical,
		Robots:        "index, follow",
		OGType:        "website",
		OGTitle:       i18n.T(lang, "landing.hero.heading"),
		OGDescription: description,
		OGURL:         canonical,
		OGImage:       ogImage,
		OGLocale:      ogLocale(lang),
		SiteName:      i18n.T(lang, "brand"),
		TwitterCard:   "summary",
		JSONLD:        landingJSONLD(title, description, canonical, lang),
	}
}

func landingJSONLD(name, description, url, lang string) template.JS {
	payload := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebSite",
		"name":        name,
		"description": description,
		"url":         url,
		"inLanguage":  lang,
	}
	return mustJSONLD(payload)
}

func postJSONLD(seo *SEOView, post store.Post, lang string) template.JS {
	payload := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "BlogPosting",
		"headline":    post.Title,
		"description": seo.Description,
		"url":         seo.CanonicalURL,
		"inLanguage":  lang,
		"author": map[string]string{
			"@type": "Person",
			"name":  seo.Author,
		},
		"publisher": map[string]string{
			"@type": "Organization",
			"name":  i18n.T(lang, "brand"),
		},
		"mainEntityOfPage": map[string]string{
			"@type": "WebPage",
			"@id":   seo.CanonicalURL,
		},
	}
	if seo.ArticlePublished != "" {
		payload["datePublished"] = seo.ArticlePublished
	}
	if seo.ArticleModified != "" {
		payload["dateModified"] = seo.ArticleModified
	}
	if seo.OGImage != "" {
		payload["image"] = seo.OGImage
	}
	return mustJSONLD(payload)
}

func blogJSONLD(name, description, url, authorName, lang, image string) template.JS {
	payload := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Blog",
		"name":        name,
		"description": description,
		"url":         url,
		"inLanguage":  lang,
		"author": map[string]string{
			"@type": "Person",
			"name":  authorName,
		},
	}
	if image != "" {
		payload["image"] = image
	}
	return mustJSONLD(payload)
}

func mustJSONLD(payload map[string]any) template.JS {
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatRFC822(t time.Time) string {
	return t.UTC().Format(time.RFC1123Z)
}

func blogSiteURL(r *http.Request, baseDomain string, owner store.User) string {
	return absolutePublicURL(r, baseDomain, owner, "/")
}

func appSiteURL(r *http.Request, baseDomain string) string {
	return appBaseURL(r, baseDomain) + "/"
}

func robotsTxtForBlog(r *http.Request, baseDomain string, owner store.User) string {
	sitemap := absolutePublicURL(r, baseDomain, owner, "/sitemap.xml")
	return fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s\n", sitemap)
}

func robotsTxtForApp(r *http.Request, baseDomain string) string {
	sitemap := appBaseURL(r, baseDomain) + "/sitemap.xml"
	return fmt.Sprintf(`User-agent: *
Allow: /
Disallow: /write
Disallow: /inbox
Disallow: /settings
Disallow: /login
Disallow: /signup
Disallow: /api/

Sitemap: %s
`, sitemap)
}
