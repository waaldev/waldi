package importblogir

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Not-Like-Us": "not-like-us",
		"قصه-های-من-و-مام-بزرگ-29": "قصه-های-من-و-مام-بزرگ-29",
		"  spaced   out  ":         "spaced-out",
		"":                         "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRewriteProtocolRelativeURLs(t *testing.T) {
	in := `<p><a href="//bayanbox.ir/info/x"><img src="//bayanbox.ir/view/x.jpg"></a></p>`
	want := `<p><a href="https://bayanbox.ir/info/x"><img src="https://bayanbox.ir/view/x.jpg"></a></p>`
	if got := rewriteProtocolRelativeURLs(in); got != want {
		t.Errorf("rewriteProtocolRelativeURLs = %q, want %q", got, want)
	}
}

func TestParseTime(t *testing.T) {
	got, err := ParseTime("2025-06-22 19:04:48.223637")
	if err != nil {
		t.Fatalf("ParseTime: %v", err)
	}
	if got.Year() != 2025 || got.Month() != 6 || got.Day() != 22 {
		t.Errorf("got %v", got)
	}
	if _, err := ParseTime(""); err == nil {
		t.Error("expected error for empty time")
	}
}

func TestLoadExport(t *testing.T) {
	exp, err := LoadExport("testdata/sample.xml")
	if err != nil {
		t.Fatalf("LoadExport: %v", err)
	}
	if len(exp.Posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(exp.Posts))
	}
	p := exp.Posts[0]
	if p.Title != "\n\t\t\t\t\tHello World\n\t\t\t\t" {
		t.Errorf("title = %q", p.Title)
	}
	if p.Number != 1 {
		t.Errorf("number = %d", p.Number)
	}
}
