package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"waldi/internal/store"
)

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if blog := s.isBlogHost(r.Context(), r.Host); blog != nil && s.store != nil {
		owner, err := s.store.UserByUsername(r.Context(), blog.Username)
		if err == nil {
			_, _ = fmt.Fprint(w, robotsTxtForBlog(r, s.baseDomain, owner))
			return
		}
	}
	_, _ = fmt.Fprint(w, robotsTxtForApp(r, s.baseDomain))
}

func (s *Server) handleBlogFeed(w http.ResponseWriter, r *http.Request) {
	blog := s.isBlogHost(r.Context(), r.Host)
	if blog == nil || s.store == nil {
		http.NotFound(w, r)
		return
	}

	owner, err := s.store.UserByUsername(r.Context(), blog.Username)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading feed author", "err", err)
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return
	}

	posts, err := s.store.PublishedPostsByUsername(r.Context(), blog.Username, rssPostLimit, store.PageCursor{})
	if err != nil {
		s.logger.Error("loading feed posts", "err", err)
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return
	}

	displayName := publicDisplayName(owner)
	siteURL := blogSiteURL(r, s.baseDomain, owner)
	feedURL := absolutePublicURL(r, s.baseDomain, owner, "/feed.xml")
	description := blogDescription(blogPageLang(owner), displayName, owner.Bio)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:content="http://purl.org/rss/1.1/modules/content/">`)
	b.WriteString("<channel>")
	fmt.Fprintf(&b, "<title>%s</title>", xmlEscape(displayName))
	fmt.Fprintf(&b, "<link>%s</link>", xmlEscape(siteURL))
	fmt.Fprintf(&b, "<description>%s</description>", xmlEscape(description))
	fmt.Fprintf(&b, `<atom:link href="%s" rel="self" type="application/rss+xml"/>`, xmlEscape(feedURL))
	b.WriteString("<language>" + xmlEscape(blogPageLang(owner)) + "</language>")

	for _, post := range posts {
		itemURL := absolutePublicURL(r, s.baseDomain, owner, "/"+post.Slug)
		itemDesc := xmlEscape(postExcerpt(post.HTML, metaDescriptionMax))
		fmt.Fprintf(&b, "<item>")
		fmt.Fprintf(&b, "<title>%s</title>", xmlEscape(post.Title))
		fmt.Fprintf(&b, "<link>%s</link>", xmlEscape(itemURL))
		fmt.Fprintf(&b, "<guid isPermaLink=\"true\">%s</guid>", xmlEscape(itemURL))
		if post.PublishedAt != nil {
			fmt.Fprintf(&b, "<pubDate>%s</pubDate>", xmlEscape(formatRFC822(*post.PublishedAt)))
		}
		if itemDesc != "" {
			fmt.Fprintf(&b, "<description>%s</description>", itemDesc)
		}
		if post.HTML != "" {
			fmt.Fprintf(&b, "<content:encoded>%s</content:encoded>", cdataEscape(post.HTML))
		}
		b.WriteString("</item>")
	}

	b.WriteString("</channel></rss>")
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func (s *Server) handleBlogSitemap(w http.ResponseWriter, r *http.Request) {
	blog := s.isBlogHost(r.Context(), r.Host)
	if blog == nil || s.store == nil {
		http.NotFound(w, r)
		return
	}

	owner, err := s.store.UserByUsername(r.Context(), blog.Username)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading sitemap author", "err", err)
		http.Error(w, "sitemap unavailable", http.StatusInternalServerError)
		return
	}

	posts, err := s.store.PublishedPostsByUsername(r.Context(), blog.Username, sitemapPostLimit, store.PageCursor{})
	if err != nil {
		s.logger.Error("loading sitemap posts", "err", err)
		http.Error(w, "sitemap unavailable", http.StatusInternalServerError)
		return
	}

	homeURL := blogSiteURL(r, s.baseDomain, owner)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	writeSitemapURL(&b, homeURL, owner.CreatedAt)
	for _, post := range posts {
		postURL := absolutePublicURL(r, s.baseDomain, owner, "/"+post.Slug)
		lastMod := post.UpdatedAt
		if post.PublishedAt != nil {
			lastMod = *post.PublishedAt
		}
		writeSitemapURL(&b, postURL, lastMod)
	}
	b.WriteString("</urlset>")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func (s *Server) handleBlogSitemapOrApp(w http.ResponseWriter, r *http.Request) {
	if s.isBlogHost(r.Context(), r.Host) != nil {
		s.handleBlogSitemap(w, r)
		return
	}
	s.handleAppSitemap(w, r)
}

func (s *Server) handleAppSitemap(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	writeSitemapURL(&b, strings.TrimSuffix(appSiteURL(r, s.baseDomain), "/"), time.Now())
	b.WriteString("</urlset>")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// cdataEscape wraps html in a CDATA section, escaping any literal "]]>"
// so it can't prematurely terminate the section.
func cdataEscape(html string) string {
	escaped := strings.ReplaceAll(html, "]]>", "]]]]><![CDATA[>")
	return "<![CDATA[" + escaped + "]]>"
}

func writeSitemapURL(b *strings.Builder, loc string, lastMod time.Time) {
	fmt.Fprintf(b, "<url><loc>%s</loc>", xmlEscape(loc))
	if !lastMod.IsZero() {
		fmt.Fprintf(b, "<lastmod>%s</lastmod>", xmlEscape(lastMod.UTC().Format("2006-01-02")))
	}
	b.WriteString("</url>")
}
