package web

import "testing"

func TestPasswordHash(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !checkPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to match hash")
	}
	if checkPassword(hash, "wrong password") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestValidEmail(t *testing.T) {
	valid := []string{"a@b.co", "negar@waldi.blog", "first.last+tag@sub.example.com"}
	for _, email := range valid {
		if !validEmail(email) {
			t.Errorf("expected %q to be valid", email)
		}
	}
	invalid := []string{"", "asdasd", "asdasd@asdasd", "@example.com", "foo@", "foo bar@example.com", "foo@example .com"}
	for _, email := range invalid {
		if validEmail(email) {
			t.Errorf("expected %q to be invalid", email)
		}
	}
}

func TestNewSessionToken(t *testing.T) {
	a, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == "" || b == "" {
		t.Fatal("expected non-empty tokens")
	}
	if a == b {
		t.Fatal("expected unique tokens")
	}
}
