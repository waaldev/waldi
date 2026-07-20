package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"waldi/internal/jobs"
	"waldi/internal/store"
)

const (
	// minCompletedReadingsForLetters gates letter-sending on having actually
	// read a few posts first — you write to a writer after reading them.
	minCompletedReadingsForLetters = 3
	// maxLettersPerDay caps how many letters one account can send in a day.
	maxLettersPerDay = 5
)

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderInboxError(w, r, "inbox.error.db")
		return
	}

	cursor, err := parsePageCursor(r)
	if err != nil {
		s.renderInboxError(w, r, "error.bad_cursor")
		return
	}
	raw, err := s.store.LettersForUser(r.Context(), user.ID, time.Now().Add(-inboxWindow), inboxPageSize+1, cursor)
	if err != nil {
		s.logger.Error("loading inbox", "err", err)
		s.renderInboxError(w, r, "inbox.error.list")
		return
	}
	letters, hasMore := trimPage(raw, inboxPageSize)
	var stats []store.PostStats
	if !cursor.Active() {
		stats, err = s.store.PostStatsForUser(r.Context(), user.ID, time.Now().Add(-24*time.Hour), 10)
		if err != nil {
			s.logger.Error("loading inbox stats", "err", err)
		}
	}

	pd := s.newPageData(r, user)
	views := letterViews(r, s.baseDomain, letters, pd.Lang)
	pd.Title = pd.T("inbox.title")
	pd.SEO = noindexSEO()
	pd.Inbox = &InboxView{
		Letters:  views,
		Stats:    statsViews(pd.Lang, stats),
		Empty:    len(views) == 0 && len(stats) == 0,
		OlderURL: lettersOlderURL("/inbox", letters, hasMore),
	}
	s.renderer.Render(w, "inbox.html", pd)
}

func (s *Server) handleInboxArchive(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderInboxError(w, r, "inbox.error.db")
		return
	}

	cursor, err := parsePageCursor(r)
	if err != nil {
		s.renderInboxError(w, r, "error.bad_cursor")
		return
	}
	raw, err := s.store.LettersArchiveForUser(r.Context(), user.ID, letterArchivePageSize+1, cursor)
	if err != nil {
		s.logger.Error("loading letter archive", "err", err)
		s.renderInboxError(w, r, "inbox.error.list")
		return
	}
	letters, hasMore := trimPage(raw, letterArchivePageSize)

	pd := s.newPageData(r, user)
	views := letterViews(r, s.baseDomain, letters, pd.Lang)
	pd.Title = pd.T("inbox.archive.title")
	pd.SEO = noindexSEO()
	pd.Inbox = &InboxView{
		Letters:  views,
		Empty:    len(views) == 0,
		OlderURL: lettersOlderURL("/inbox/archive", letters, hasMore),
	}
	s.renderer.Render(w, "inbox-archive.html", pd)
}

func (s *Server) handleLetter(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderInboxError(w, r, "inbox.error.db")
		return
	}

	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	letter, err := s.store.LetterForUser(r.Context(), id, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading letter", "err", err)
		s.renderInboxError(w, r, "inbox.error.letter")
		return
	}
	if err := s.store.MarkLetterRead(r.Context(), id, user.ID); err != nil {
		s.logger.Error("marking letter read", "err", err)
	}

	pd := s.newPageData(r, user)
	view := letterView(r, s.baseDomain, letter, pd.Lang)
	view.Read = true
	pd.Title = pd.T("letter.page.title", view.FromWriterLabel)
	pd.SEO = noindexSEO()
	pd.Inbox = &InboxView{Letter: &view}
	s.renderer.Render(w, "letter.html", pd)
}

func (s *Server) handleCreateLetter(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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

	postID, err := strconv.ParseInt(r.FormValue("post_id"), 10, 64)
	if err != nil || postID < 1 {
		http.Error(w, "bad post", http.StatusBadRequest)
		return
	}
	post, err := s.store.PublishedPostByID(r.Context(), postID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading letter post", "err", err)
		http.Error(w, "letter failed", http.StatusInternalServerError)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if len([]rune(body)) < 2 {
		http.Error(w, "letter is empty", http.StatusBadRequest)
		return
	}
	if len([]rune(body)) > 5000 {
		http.Error(w, "letter is too long", http.StatusBadRequest)
		return
	}
	if post.UserID == user.ID {
		redirect(w, r, redirectBack(r))
		return
	}

	completed, err := s.store.CompletedReadingsCount(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("checking completed readings", "err", err)
		http.Error(w, "letter failed", http.StatusInternalServerError)
		return
	}
	if completed < minCompletedReadingsForLetters {
		http.Error(w, "read a few more posts before writing letters", http.StatusForbidden)
		return
	}

	sentToday, err := s.store.LettersSentSince(r.Context(), user.ID, time.Now().Add(-24*time.Hour))
	if err != nil {
		s.logger.Error("checking letter rate limit", "err", err)
		http.Error(w, "letter failed", http.StatusInternalServerError)
		return
	}
	if sentToday >= maxLettersPerDay {
		http.Error(w, "too many letters today, try again tomorrow", http.StatusTooManyRequests)
		return
	}

	if _, err := s.store.CreateLetter(r.Context(), postID, user.ID, post.UserID, body); err != nil {
		s.logger.Error("creating letter", "err", err)
		http.Error(w, "letter failed", http.StatusInternalServerError)
		return
	}
	redirect(w, r, withQueryParam(redirectBack(r), "letter", "sent"))
}

func (s *Server) renderInboxError(w http.ResponseWriter, r *http.Request, messageKey string) {
	pd := s.newPageData(r, currentUser(r))
	pd.Title = pd.T("inbox.title")
	pd.SEO = noindexSEO()
	pd.Inbox = &InboxView{Error: pd.T(messageKey)}
	s.renderer.RenderStatus(w, http.StatusBadRequest, "inbox.html", pd)
}

func letterViews(r *http.Request, baseDomain string, letters []store.Letter, lang string) []LetterView {
	views := make([]LetterView, 0, len(letters))
	for _, letter := range letters {
		views = append(views, letterView(r, baseDomain, letter, lang))
	}
	return views
}

func letterView(r *http.Request, baseDomain string, letter store.Letter, lang string) LetterView {
	return LetterView{
		ID:              letter.ID,
		PostID:          letter.PostID,
		PostTitle:       letter.PostTitle,
		PostSlug:        letter.PostSlug,
		FromUsername:    letter.FromUsername,
		FromWriterLabel: writerLabel(letter.FromAuthorName, letter.FromDisplayName, letter.FromUsername),
		FromBlogURL:     PublicBlogURL(r, baseDomain, letter.FromUsername, "/"),
		Body:            letter.Body,
		CreatedAt:       formatDate(letter.CreatedAt, lang),
		Read:            letter.ReadAt != nil,
	}
}

func statsViews(lang string, stats []store.PostStats) []StatsView {
	views := make([]StatsView, 0, len(stats))
	for _, stat := range stats {
		views = append(views, StatsView{
			PostTitle: stat.PostTitle,
			Sentence:  jobs.DigestSentence(lang, stat),
		})
	}
	return views
}
