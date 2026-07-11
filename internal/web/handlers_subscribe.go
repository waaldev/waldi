package web

import (
	"net/http"
	"strconv"
	"strings"
)

// handleSubscribe stores an anonymous visitor's email capture and redirects
// back to the post they submitted it from. The redirect target is derived
// from a server-side post lookup (not the Referer header) to avoid an open
// redirect via a spoofed header.
func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?subscribed=1", http.StatusSeeOther)
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	username := strings.TrimSpace(r.FormValue("username"))
	postIDStr := strings.TrimSpace(r.FormValue("post_id"))

	redirectTo := s.subscribeRedirectTarget(r, postIDStr)

	if email == "" || !strings.Contains(email, "@") || s.store == nil {
		http.Redirect(w, r, redirectTo, http.StatusSeeOther)
		return
	}

	var postID *int64
	if postIDStr != "" {
		if id, err := strconv.ParseInt(postIDStr, 10, 64); err == nil {
			postID = &id
		}
	}

	if err := s.store.CreateEmailCapture(r.Context(), email, username, postID); err != nil {
		s.logger.Error("creating email capture", "err", err)
		http.Redirect(w, r, redirectTo, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, redirectTo+"?subscribed=1", http.StatusSeeOther)
}

func (s *Server) subscribeRedirectTarget(r *http.Request, postIDStr string) string {
	if postIDStr != "" && s.store != nil {
		if id, err := strconv.ParseInt(postIDStr, 10, 64); err == nil {
			if post, err := s.store.PublishedPostByID(r.Context(), id); err == nil {
				return "/" + post.Slug
			}
		}
	}
	return "/"
}
