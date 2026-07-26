package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"waldi/internal/store"
)

func (s *Server) handleKeep(w http.ResponseWriter, r *http.Request) {
	s.changeKeep(w, r, true)
}

func (s *Server) handleUnkeep(w http.ResponseWriter, r *http.Request) {
	s.changeKeep(w, r, false)
}

func (s *Server) changeKeep(w http.ResponseWriter, r *http.Request, keep bool) {
	user := currentUser(r)
	if user == nil {
		redirect(w, r, s.loginURL(r, s.baseDomain, pageURL(r)))
		return
	}
	if s.store == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || postID < 1 {
		http.NotFound(w, r)
		return
	}

	if keep {
		var letterID *int64
		if raw := r.FormValue("letter_id"); raw != "" {
			if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
				letterID = &id
			}
		}
		err = s.store.Keep(r.Context(), user.ID, postID, letterID)
	} else {
		err = s.store.Unkeep(r.Context(), user.ID, postID)
	}
	if err != nil {
		s.logger.Error("changing keep", "err", err)
		http.Error(w, "keep failed", http.StatusInternalServerError)
		return
	}
	redirect(w, r, redirectSameHost(r))
}

// markKept sets CanKeep/Kept on each view with a single query, so a page
// showing many posts at once (the home feed) doesn't issue one keep lookup
// per post. The wildcard card is passed separately by callers that build it,
// since Keep is only offered on followed posts, not the daily stranger.
func (s *Server) markKept(r *http.Request, userID int64, views []PostView) {
	ids := make([]int64, len(views))
	for i, v := range views {
		ids[i] = v.ID
	}
	kept, err := s.store.KeptPostIDs(r.Context(), userID, ids)
	if err != nil {
		s.logger.Error("loading kept post ids", "err", err)
		return
	}
	for i := range views {
		views[i].CanKeep = true
		views[i].Kept = kept[views[i].ID]
	}
}

// handleKept renders a reader's private shelf of kept posts, newest keep
// first. Nobody but the reader ever sees this list or a count of it.
func (s *Server) handleKept(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}

	cursor, err := parsePageCursor(r)
	if errors.Is(err, errBadPageCursor) {
		pd := s.newPageData(r, user)
		http.Error(w, pd.T("error.bad_cursor"), http.StatusBadRequest)
		return
	}

	rawKept, err := s.store.KeptPosts(r.Context(), user.ID, publishedPageSize+1, cursor)
	if err != nil {
		s.logger.Error("loading kept posts", "err", err)
		pd := s.newPageData(r, user)
		http.Error(w, pd.T("error.home"), http.StatusInternalServerError)
		return
	}
	kept, hasMore := trimPage(rawKept, publishedPageSize)

	posts := make([]store.Post, len(kept))
	for i, kp := range kept {
		posts[i] = kp.Post
	}
	pd := s.newPageData(r, user)

	views := s.postViewsWithURLs(r, posts)
	now := time.Now()
	var postViews, letterViews []PostView
	for i, kp := range kept {
		views[i].CanKeep = true
		views[i].Kept = true
		views[i].KeptWhen = formatKeptWhen(kp.KeptAt, now, pd.Lang)
		// A post kept from a received letter is shown as that letter - the
		// sender, their words, and a link back to the letter itself - since
		// the letter, not the underlying post, is what was kept.
		if kp.SourceLetterID != nil {
			views[i].URL = fmt.Sprintf("/inbox/%d", *kp.SourceLetterID)
			views[i].FromLetter = true
			views[i].SourceLetterID = *kp.SourceLetterID
			views[i].Excerpt = kp.SourceLetterBody
			views[i].WriterLabel = writerLabel(kp.SourceLetterFromName, kp.SourceLetterFromDisplay, kp.SourceLetterFromUser)
			views[i].BlogURL = PublicBlogURL(r, s.baseDomain, kp.SourceLetterFromUser, "/")
			letterViews = append(letterViews, views[i])
		} else {
			postViews = append(postViews, views[i])
		}
	}

	pd.Title = pd.T("kept.title")
	pd.SEO = noindexSEO()
	pd.NavActive = "kept"
	pd.Kept = &KeptView{
		Posts:    postViews,
		Letters:  letterViews,
		Empty:    len(kept) == 0 && !cursor.Active(),
		OlderURL: keptOlderURL(kept, hasMore),
	}
	s.renderer.Render(w, "kept.html", pd)
}
