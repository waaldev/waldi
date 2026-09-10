package web

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingPurger struct {
	hosts chan []string
	urls  chan []string
}

func (p *recordingPurger) PurgeHosts(_ context.Context, hosts []string) error {
	p.hosts <- hosts
	return nil
}

func (p *recordingPurger) PurgeURLs(_ context.Context, urls []string) error {
	p.urls <- urls
	return nil
}

func TestBlogPublicHosts(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}
	got := s.blogPublicHosts("sara")
	want := []string{"sara.waldi.blog"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("hosts = %#v, want %#v", got, want)
	}
}

func TestBlogPublicHostsExtra(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}
	got := s.blogPublicHosts("sara", "old.example.com")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got), got)
	}
	if got[0] != "sara.waldi.blog" || got[1] != "old.example.com" {
		t.Fatalf("hosts = %#v", got)
	}
}

func TestPurgePublicCacheUsesOneHostnameRequest(t *testing.T) {
	purger := &recordingPurger{
		hosts: make(chan []string, 1),
		urls:  make(chan []string, 1),
	}
	s := &Server{
		baseDomain:    "waldi.blog",
		cdnPurger:     purger,
		customDomains: newCustomDomainCache(),
	}

	s.purgePublicCache("sara")

	select {
	case hosts := <-purger.hosts:
		if len(hosts) != 1 || hosts[0] != "sara.waldi.blog" {
			t.Fatalf("hosts = %#v, want only the writer hostname", hosts)
		}
	case <-time.After(time.Second):
		t.Fatal("hostname purge was not called")
	}

	select {
	case urls := <-purger.urls:
		t.Fatalf("unexpected URL purge: %#v", urls)
	default:
	}
}

func TestRetryCachePurge(t *testing.T) {
	attempts := 0
	err := retryCachePurgeWithDelay(context.Background(), 0, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}
