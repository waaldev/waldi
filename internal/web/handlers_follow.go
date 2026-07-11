package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"waldi/internal/store"
)

func (s *Server) handleFollow(w http.ResponseWriter, r *http.Request) {
	s.changeFollow(w, r, true)
}

func (s *Server) handleUnfollow(w http.ResponseWriter, r *http.Request) {
	s.changeFollow(w, r, false)
}

func (s *Server) changeFollow(w http.ResponseWriter, r *http.Request, follow bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	user := currentUser(r)
	if user == nil {
		redirect(w, r, s.loginURL(r, s.baseDomain, pageURL(r)))
		return
	}
	if s.store == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	target, err := s.store.UserByUsername(r.Context(), r.PathValue("username"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading follow target", "err", err)
		http.Error(w, "follow failed", http.StatusInternalServerError)
		return
	}
	if target.ID == user.ID {
		redirect(w, r, redirectSameHost(r))
		return
	}

	if follow {
		var sourcePostID *int64
		if raw := r.FormValue("source_post_id"); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err == nil && id > 0 {
				sourcePostID = &id
			}
		}
		err = s.store.Follow(r.Context(), user.ID, target.ID, sourcePostID)
	} else {
		err = s.store.Unfollow(r.Context(), user.ID, target.ID)
	}
	if err != nil {
		s.logger.Error("changing follow", "err", err)
		http.Error(w, "follow failed", http.StatusInternalServerError)
		return
	}
	redirect(w, r, redirectSameHost(r))
}

// redirectBack is an alias for redirectSameHost.
func redirectBack(r *http.Request) string {
	return redirectSameHost(r)
}

// redirectSameHost returns a relative path on the current host.
func redirectSameHost(r *http.Request) string {
	ref := strings.TrimSpace(r.Referer())
	if ref == "" {
		return "/"
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "/"
	}
	if hostWithoutPort(u.Host) != hostWithoutPort(r.Host) {
		return "/"
	}
	dest := u.EscapedPath()
	if dest == "" {
		dest = "/"
	}
	if u.RawQuery != "" {
		dest += "?" + u.RawQuery
	}
	return dest
}

// loginURL keeps login on the current host when on a writer subdomain or
// verified custom domain.
func (s *Server) loginURL(r *http.Request, baseDomain, returnTo string) string {
	returnTo = strings.TrimSpace(returnTo)
	if s.isBlogHost(r.Context(), r.Host) != nil {
		dest := "/login"
		if returnTo != "" {
			dest += "?next=" + url.QueryEscape(returnTo)
		}
		return dest
	}
	return appLoginURL(r, baseDomain, returnTo)
}
