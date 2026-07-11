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
