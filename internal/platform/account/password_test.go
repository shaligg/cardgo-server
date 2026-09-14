package account

import (
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
)

func TestPasswordAndCanonicalAccount(t *testing.T) {
	name, err := credentials(" User_1 ", "correct-password")
	if err != nil || name != "user_1" {
		t.Fatalf("canonical name: %q %v", name, err)
	}
	for _, p := range []string{"short", strings.Repeat("a", 73)} {
		if _, err := credentials("user", p); err != ErrBadRequest {
			t.Fatal("password length accepted")
		}
	}
	for _, n := range []string{"ab", "用户名字", "x y", strings.Repeat("a", 65)} {
		if _, err := credentials(n, "correct-password"); err != ErrBadRequest {
			t.Fatal("invalid account accepted")
		}
	}
	h, err := hashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if h == "correct-password" || bcrypt.CompareHashAndPassword([]byte(h), []byte("correct-password")) != nil {
		t.Fatal("invalid password hash")
	}
	if bcrypt.CompareHashAndPassword([]byte(h), []byte("wrong-password")) == nil {
		t.Fatal("wrong password accepted")
	}
}
