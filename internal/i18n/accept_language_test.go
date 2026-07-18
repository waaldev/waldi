package i18n

import "testing"

func TestLangFromAcceptLanguage(t *testing.T) {
	tests := []struct {
		header string
		want   string
		wantOk bool
	}{
		{"en", "en", true},
		{"en-GB,en;q=0.9", "en", true},
		{"fa-IR", "fa", true},
		{"da,en-GB;q=0.8,en;q=0.7", "en", true},
		{"da;q=0.9,fa;q=0.5,en;q=0.8", "en", true},
		{"en;q=0.4,fa;q=0.6", "fa", true},
		{"en;q=0", "", false},
		{"da,de;q=0.9", "", false},
		{"*", "", false},
		{"", "", false},
	}
	for _, tc := range tests {
		got, ok := LangFromAcceptLanguage(tc.header)
		if got != tc.want || ok != tc.wantOk {
			t.Errorf("LangFromAcceptLanguage(%q) = %q, %v; want %q, %v", tc.header, got, ok, tc.want, tc.wantOk)
		}
	}
}
