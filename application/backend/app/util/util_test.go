package util

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func TestContains(t *testing.T) {
	type args struct {
		a    string
		list []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Is contain 'a'", args{a: "a", list: []string{"a", "b", "c"}}, true},
		{"Is contain 'A'", args{a: "A", list: []string{"a", "b", "c"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.a, tt.args.list); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Rewritten: this used to assert that an admin is created with no password set.
func TestCreateInitUser(t *testing.T) {
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "an-actual-admin-password")
	t.Cleanup(func() { os.Setenv("ADMIN_PASSWORD", "") })

	if err := CreateInitUser(); err != nil {
		t.Fatalf("CreateInitUser: %v", err)
	}

	isExist, err := vars.UsersDB.Has([]byte("admin"), nil)
	if err != nil {
		t.Error(err.Error())
	}
	if !isExist {
		t.Error("User was not created!")
	}
}

func TestCreateJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "1231efdZF")
	token := CreateJWT("test_user")

	test_req, _ := http.NewRequest("GET", "", nil)
	test_req.AddCookie(
		&http.Cookie{
			Name:  "onlogs-cookie",
			Value: token,
		},
	)

	username, err := GetUserFromJWT(*test_req)
	if err != nil {
		t.Error(err)
	}
	if username != "test_user" {
		t.Error("Username in JWT is wrong: ", username)
	}
}

func TestGetHost(t *testing.T) {
	host, _ := os.Hostname()
	if host[len(host)-1] < 32 || host[len(host)-1] > 126 {
		host = host[:len(host)-1]
	}

	if GetHost() != host {
		t.Error("Hosts is not matching!")
	}
}

func TestGetUserFromJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "1231efdZF")

	test_req1, _ := http.NewRequest("GET", "", nil)
	test_req1.AddCookie(
		&http.Cookie{
			Name:  "onlogs-cookie",
			Value: CreateJWT("test_user"),
		},
	)
	test_req2, _ := http.NewRequest("GET", "", nil)
	test_req2.AddCookie(
		&http.Cookie{
			Name:  "onlogs-cookie",
			Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdXRob3JpemVkIjp0cnVlLCJleHAiOjE3NjgwNzcxMjcsInVzZXIiOiJ0ZXN0X3VzZXIifQ.KOplTT3L13v68qda3Z0_F0ZYjqn__wq2kcji7dLnuUE",
		},
	)

	test_req3, _ := http.NewRequest("GET", "", nil)

	username, _ := GetUserFromJWT(*test_req1)
	if username != "test_user" {
		t.Error("Username in JWT is wrong: ", username)
	}

	_, err := GetUserFromJWT(*test_req2)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Error("Token should be expired")
	}

	_, err = GetUserFromJWT(*test_req3)
	if err.Error() != "401 - Unauthorized!" {
		t.Error("Req should be unauthorized")
	}

}
