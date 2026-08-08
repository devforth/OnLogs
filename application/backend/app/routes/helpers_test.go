package routes

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devforth/OnLogs/app/util"
)

// An empty user means no cookie at all, which is what an unauthenticated
// request and a DISABLE_AUTH request both look like.
func authedRequest(t *testing.T, user, method, target string, body any) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshalling the request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	if user != "" {
		req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT(user)})
	}
	return req
}

func call(t *testing.T, handler http.HandlerFunc, req *http.Request) (int, []byte) {
	t.Helper()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	body, _ := io.ReadAll(rr.Result().Body)
	return rr.Result().StatusCode, body
}
