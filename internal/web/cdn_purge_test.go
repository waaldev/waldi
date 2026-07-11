package web

import "testing"

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

func TestCDNPurgePrefixes(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}

	got := s.cdnPurgePrefixes("sara")
	want := []string{
		"waldi.blog/",
		"sara.waldi.blog/",
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("prefix[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCDNPurgePrefixesWithRemovedDomain(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}
	got := s.cdnPurgePrefixes("sara", "blog.example.com")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %#v", len(got), got)
	}
	if got[2] != "blog.example.com/" {
		t.Fatalf("prefix[2] = %q", got[2])
	}
}

func TestCDNPurgePrefixesHomeOnly(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}
	got := s.cdnPurgePrefixes("")
	if len(got) != 1 || got[0] != "waldi.blog/" {
		t.Fatalf("prefixes = %#v", got)
	}
}

func TestCDNPurgeURLs(t *testing.T) {
	got := (&Server{}).cdnPurgeURLs([]string{"sara.waldi.blog"})
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4: %#v", len(got), got)
	}
	if got[1] != "https://sara.waldi.blog/feed.xml" {
		t.Fatalf("feed url = %q", got[1])
	}
}
