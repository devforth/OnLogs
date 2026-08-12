package util

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt/v5"
)

func requestWithCookie(value string) http.Request {
	req, _ := http.NewRequest("GET", "", nil)
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: value})
	return *req
}

func TestGetUserFromJWTRejectsUnsignedTokens(t *testing.T) {
	os.Setenv("JWT_SECRET", "a-real-secret-value-for-the-test")

	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"user":       "admin",
		"authorized": true,
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	user, err := GetUserFromJWT(requestWithCookie(unsigned))
	if err == nil {
		t.Fatalf("alg=none token accepted as user %q", user)
	}
}

func TestGetUserFromJWTRejectsTokenWithoutExpiry(t *testing.T) {
	secret := "a-real-secret-value-for-the-test"
	os.Setenv("JWT_SECRET", secret)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":       "admin",
		"authorized": true,
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	user, err := GetUserFromJWT(requestWithCookie(signed))
	if err == nil {
		t.Fatalf("token without exp accepted as user %q", user)
	}
}

func TestGetUserFromJWTRejectsTokenWithoutUser(t *testing.T) {
	secret := "a-real-secret-value-for-the-test"
	os.Setenv("JWT_SECRET", secret)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"authorized": true,
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	if user, err := GetUserFromJWT(requestWithCookie(signed)); err == nil {
		t.Fatalf("token without a user claim accepted as %q", user)
	}
}

func TestGetUserFromJWTRejectsEverythingWhenTheSecretIsEmpty(t *testing.T) {
	os.Setenv("JWT_SECRET", "")
	t.Cleanup(func() { os.Setenv("JWT_SECRET", "1231efdZF") })

	forged := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":       "admin",
		"authorized": true,
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	signed, err := forged.SignedString([]byte(""))
	if err != nil {
		t.Fatal(err)
	}

	if user, err := GetUserFromJWT(requestWithCookie(signed)); err == nil {
		t.Fatalf("cookie forged with an empty key authenticated as %q", user)
	}
}

func TestGenerateJWTSecretIsNotPredictable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		s := GenerateJWTSecret()
		if len(s) < 25 {
			t.Fatalf("token too short: %q", s)
		}
		if seen[s] {
			t.Fatalf("duplicate token generated: %q", s)
		}
		seen[s] = true
	}
}

func TestGetUserFromJWTRejectsADeletedAccount(t *testing.T) {
	secret := "a-real-secret-value-for-the-test"
	os.Setenv("JWT_SECRET", secret)

	login := "about-to-be-deleted"
	if err := vars.UsersDB.Put([]byte(login), []byte("irrelevant"), nil); err != nil {
		t.Fatal(err)
	}

	signed := CreateJWT(login)
	if user, err := GetUserFromJWT(requestWithCookie(signed)); err != nil || user != login {
		t.Fatalf("the token should work while the account exists: %q %v", user, err)
	}

	if err := vars.UsersDB.Delete([]byte(login), nil); err != nil {
		t.Fatal(err)
	}

	if user, err := GetUserFromJWT(requestWithCookie(signed)); err == nil {
		t.Fatalf("a deleted account still authenticates as %q; deleting a user revokes nothing", user)
	}
}
