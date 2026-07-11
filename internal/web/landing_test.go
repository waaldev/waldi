package web

import "testing"

func TestLandingSamplePosts(t *testing.T) {
	got := landingSamplePosts()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, p := range got {
		if p.Title == "" || p.URL == "" {
			t.Fatalf("sample post = %#v", p)
		}
	}
	p := got[0]
	if p.Username != "لیلی" {
		t.Fatalf("Username = %q", p.Username)
	}
	if p.Lang != "fa" || p.Dir != "rtl" {
		t.Fatalf("lang/dir = %q / %q", p.Lang, p.Dir)
	}
	p2 := got[1]
	if p2.Username != "Amin" {
		t.Fatalf("Username = %q", p2.Username)
	}
	if p2.Lang != "en" || p2.Dir != "ltr" {
		t.Fatalf("lang/dir = %q / %q", p2.Lang, p2.Dir)
	}
}
