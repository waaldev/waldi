package web

import (
	"testing"
	"time"
)

func TestSignupFormTimely(t *testing.T) {
	issued := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	token := signupFormToken(issued)
	cases := []struct {
		name  string
		token string
		at    time.Time
		want  bool
	}{
		{"human pace", token, issued.Add(40 * time.Second), true},
		{"too fast", token, issued.Add(time.Second), false},
		{"stale", token, issued.Add(25 * time.Hour), false},
		{"missing", "", issued.Add(time.Minute), false},
		{"forged", "1762765200.deadbeef", issued.Add(time.Minute), false},
	}
	for _, c := range cases {
		if got := signupFormTimely(c.token, c.at); got != c.want {
			t.Fatalf("%s: signupFormTimely = %v, want %v", c.name, got, c.want)
		}
	}
}
