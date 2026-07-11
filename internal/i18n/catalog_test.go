package i18n

import "testing"

func TestTUsesRequestedLanguage(t *testing.T) {
	if got := T("en", "nav.settings"); got != "Settings" {
		t.Fatalf("got %q, want Settings", got)
	}
	if got := T("fa", "nav.settings"); got != "تنظیمات" {
		t.Fatalf("got %q, want تنظیمات", got)
	}
}

func TestTMissingKeyReturnsKey(t *testing.T) {
	const key = "definitely.missing.key.xyz"
	if got := T("en", key); got != key {
		t.Fatalf("got %q, want raw key", got)
	}
}

func TestReaderLang(t *testing.T) {
	if got := ReaderLang("en"); got != "en" {
		t.Fatalf("ReaderLang(en) = %q", got)
	}
	if got := ReaderLang("fa"); got != "fa" {
		t.Fatalf("ReaderLang(fa) = %q", got)
	}
	if got := ReaderLang("de"); got != Default {
		t.Fatalf("ReaderLang(de) = %q, want %q", got, Default)
	}
}
