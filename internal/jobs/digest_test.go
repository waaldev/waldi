package jobs

import (
	"strings"
	"testing"
	"time"
	"waldi/internal/store"
)

func TestDigestSentence(t *testing.T) {
	stat := store.PostStats{
		PostTitle: "اولین نوشته",
		Readers:   142,
		Completed: 89,
		Follows:   6,
		Letters:   2,
	}

	got := DigestSentence("fa", stat)
	for _, part := range []string{"142", "89", "6", "2"} {
		if !strings.Contains(got, part) {
			t.Fatalf("DigestSentence(fa) = %q, missing %q", got, part)
		}
	}

	gotEN := DigestSentence("en", stat)
	for _, part := range []string{"142", "89", "6", "2"} {
		if !strings.Contains(gotEN, part) {
			t.Fatalf("DigestSentence(en) = %q, missing %q", gotEN, part)
		}
	}
	if got == gotEN {
		t.Fatalf("DigestSentence(fa) and DigestSentence(en) should differ, got %q for both", got)
	}
}

func TestDigestSentenceOmitsZeroClauses(t *testing.T) {
	stat := store.PostStats{PostTitle: "اولین نوشته", Readers: 142}

	got := DigestSentence("en", stat)
	if !strings.Contains(got, "142") {
		t.Fatalf("DigestSentence(en) = %q, missing reader count", got)
	}
	for _, absent := range []string{"finished", "followed", "letters"} {
		if strings.Contains(got, absent) {
			t.Fatalf("DigestSentence(en) = %q, should omit zero-value clause %q", got, absent)
		}
	}
}

func TestDigestEligible(t *testing.T) {
	verified := time.Now()
	if !digestEligible(store.User{Email: "a@b.com", EmailVerifiedAt: &verified}) {
		t.Fatal("verified user with email should be eligible")
	}
	if digestEligible(store.User{Email: "a@b.com"}) {
		t.Fatal("unverified user should not be eligible")
	}
	if digestEligible(store.User{EmailVerifiedAt: &verified}) {
		t.Fatal("empty email should not be eligible")
	}
}
