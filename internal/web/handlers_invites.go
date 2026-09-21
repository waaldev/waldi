package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
	"waldi/internal/i18n"
	"waldi/internal/mail"
	"waldi/internal/store"
)

type InvitesView struct {
	Rows []InviteRowView
}

type InviteRowView struct {
	Link           string
	Used           bool
	Name           string
	BlogURL        string
	Joined         string
	Following      bool
	FirstPostTitle string
	FirstPostURL   string
}

func (s *Server) handleInvites(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape("/invites"), http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}

	pd := s.newPageData(r, user)
	pd.Title = pd.T("invites.title")
	pd.SEO = noindexSEO()
	pd.Invites = s.invitesViewFor(r, *user, pd.Lang)
	s.renderer.Render(w, "invites.html", pd)
}

func (s *Server) invitesViewFor(r *http.Request, user store.User, lang string) *InvitesView {
	view := &InvitesView{}
	if s.store == nil || !user.EmailVerified() {
		return view
	}
	if err := s.store.EnsureUserInvitations(r.Context(), user.ID); err != nil {
		s.logger.Error("ensuring user invitations", "err", err, "user_id", user.ID)
	}
	invitations, err := s.store.UserInvitations(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("listing user invitations", "err", err, "user_id", user.ID)
		return view
	}

	base := appBaseURL(r, s.baseDomain)
	now := time.Now()
	for _, inv := range invitations {
		row := InviteRowView{Link: base + "/signup?invite=" + url.QueryEscape(inv.Code)}
		if inv.UsedAt != nil && inv.Invitee != nil {
			row.Used = true
			row.Name = writerLabelFromUser(*inv.Invitee)
			row.BlogURL = PublicBlogURLForOwner(r, s.baseDomain, *inv.Invitee, "/")
			row.Joined = formatInviteJoined(*inv.UsedAt, now, lang)
			row.Following = inv.Following
			if inv.FirstPost != nil {
				row.FirstPostTitle = inv.FirstPost.Title
				row.FirstPostURL = PublicBlogURLForOwner(r, s.baseDomain, *inv.Invitee, "/"+inv.FirstPost.Slug)
			}
		} else if inv.UsedAt != nil {
			continue
		}
		view.Rows = append(view.Rows, row)
	}
	return view
}

func formatInviteJoined(t, now time.Time, lang string) string {
	switch d := daysBetween(t.In(now.Location()), now); d {
	case 0:
		return i18n.T(lang, "invites.joined.today")
	case 1:
		return i18n.T(lang, "invites.joined.yesterday")
	default:
		return i18n.T(lang, "invites.joined.days", d)
	}
}

func (s *Server) notifyInviterOfFirstPost(r *http.Request, author store.User, post store.Post) {
	if s.store == nil || post.Type != store.PostTypePost {
		return
	}
	inviter, err := s.store.ClaimFirstPostNotification(r.Context(), author.ID)
	if errors.Is(err, store.ErrNotFound) {
		return
	}
	if err != nil {
		s.logger.Error("claiming first post notification", "err", err, "user_id", author.ID)
		return
	}
	if s.mailer == nil || inviter.Email == "" {
		return
	}

	title := post.Title
	if title == "" {
		title = i18n.T(inviter.Locale, "invites.untitled")
	}
	subject, plain, htmlBody := mail.InviteePublishedEmail(inviter.Locale, writerLabelFromUser(author), title,
		PublicBlogURLForOwner(r, s.baseDomain, author, "/"+post.Slug))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), verifyMailTimeout)
		defer cancel()
		if err := s.mailer.SendHTML(ctx, inviter.Email, subject, plain, htmlBody, mail.BrandName(inviter.Locale)); err != nil {
			s.logger.Error("sending invitee published email", "err", err, "inviter_id", inviter.ID)
		}
	}()
}
