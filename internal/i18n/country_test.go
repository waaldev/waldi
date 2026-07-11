package i18n

import "testing"

func TestLangFromCountry(t *testing.T) {
	tests := []struct {
		country string
		want    string
		wantOk  bool
	}{
		{"IR", "fa", true},
		{"AF", "fa", true},
		{"US", "en", true},
		{"GB", "en", true},
		{"", "", false},
		{"XX", "", false},
		{"T1", "", false},
	}
	for _, tc := range tests {
		got, ok := LangFromCountry(tc.country)
		if got != tc.want || ok != tc.wantOk {
			t.Errorf("LangFromCountry(%q) = (%q, %v), want (%q, %v)", tc.country, got, ok, tc.want, tc.wantOk)
		}
	}
}
