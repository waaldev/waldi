package web

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"waldi/internal/store"
)

const notifyTimeout = 10 * time.Second

func (s *Server) notifySignup(r *http.Request, user store.User) {
	if s.notifier == nil {
		return
	}
	text := fmt.Sprintf("New signup: %s (%s)\n%s", user.Username, user.Email, PublicBlogURLForOwner(r, s.baseDomain, user, "/"))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		s.notifier.Notify(ctx, text)
	}()
}

func (s *Server) notifyWriteRequest(user store.User, blogLink, note string) {
	if s.notifier == nil {
		return
	}
	text := fmt.Sprintf("Write request: %s (%s)\n%s\n%s", user.Username, user.Email, blogLink, note)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		s.notifier.Notify(ctx, text)
	}()
}

func (s *Server) notifyPublish(r *http.Request, user store.User, post store.Post) {
	if s.notifier == nil {
		return
	}
	title := post.Title
	if title == "" {
		title = "(untitled)"
	}
	text := fmt.Sprintf("Post published: %s\nby %s - %s", title, user.Username, PublicBlogURLForOwner(r, s.baseDomain, user, "/"+post.Slug))
	if s.store != nil {
		held, err := s.store.StrangerHold(r.Context(), user.ID)
		if err != nil {
			s.logger.Error("checking stranger hold", "err", err, "user_id", user.ID)
		} else if held {
			text += fmt.Sprintf("\nInvited writer, held from strangers. Release: /release %s", user.Username)
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		s.notifier.Notify(ctx, text)
	}()
}
