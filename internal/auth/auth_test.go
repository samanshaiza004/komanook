package auth

import (
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
)

func TestPasswordPolicyAndVerification(t *testing.T) {
	for _, password := range []string{"short", strings.Repeat("界", 25)} {
		if _, err := Hash([]byte(password)); err == nil {
			t.Fatal("accepted invalid password length")
		}
	}
	hash, err := Hash([]byte("a quiet place to read"))
	if err != nil {
		t.Fatal(err)
	}
	if cost, _ := bcrypt.Cost(hash); cost != Cost {
		t.Fatalf("cost=%d", cost)
	}
	if !Verify(hash, []byte("a quiet place to read")) || Verify(hash, []byte("a different password")) {
		t.Fatal("incorrect password verification")
	}
	if cost, err := bcrypt.Cost(dummyHash); err != nil || cost != Cost {
		t.Fatalf("dummy hash must match cost: %v", err)
	}
	if Verify(nil, []byte("anything")) {
		t.Fatal("unknown account authenticated")
	}
}
func TestEmailPolicy(t *testing.T) {
	got, err := Email(" Reader@Example.com ")
	if err != nil || got != "reader@example.com" {
		t.Fatal(got, err)
	}
	for _, value := range []string{"Reader <reader@example.com>", "invalid", "a\n@b.com"} {
		if _, err := Email(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
