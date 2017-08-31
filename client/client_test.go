package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// some servers definitions for diffrent kind of tests
var (
	noopHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	noopServer  = httptest.NewServer(noopHandler)

	internalServerErrorHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&ErrorResponse{"no key"})
	})
	internalServerErrorServer = httptest.NewServer(internalServerErrorHandler)

	timeoutHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(1 * time.Second) })
	timeoutServer  = httptest.NewServer(timeoutHandler)
)

func checkMethodAndPath(t *testing.T, r *http.Request, method string, path string) {
	if r.Method != method {
		t.Fatalf("method %s not found", method)
		return
	}
	if r.URL.Path != "/"+DefaultVersion+path {
		t.Fatalf("invalid url path %s", r.URL.Path)
		return
	}
}

func TestMain(m *testing.M) {
	defer noopServer.Close()
	defer internalServerErrorServer.Close()
	os.Exit(m.Run())
}

func TestCheckKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkMethodAndPath(t, r, http.MethodGet, "/account/status")
		json.NewEncoder(w).Encode(&AccountStatusResponse{})
	}))
	defer ts.Close()

	if err := New(ts.URL, "test-key").CheckKey(); err != nil {
		t.Fatal(err)
	}
}

func TestSetKey(t *testing.T) {
	c := New("", "")
	if c.SetKey("test-api-key"); c.key != "test-api-key" {
		t.Fatalf("invalid key")
	}
}

func TestBasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if username, password, ok := r.BasicAuth(); !ok || username != "test-key" || password != "" {
			t.Fatalf("invalid basic auth")
		}
	}))
	defer ts.Close()

	if _, err := New(ts.URL, "test-key").post(context.Background(), "/", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestUserAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != defaultUserAgent {
			t.Fatalf("invalid user agent")
		}
	}))
	defer ts.Close()

	if _, err := New(ts.URL, "test-key").get(context.Background(), "/", nil); err != nil {
		t.Fatal(err)
	}
}

func TestResponseStatusNotOk(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	if _, err := New(ts.URL, "").get(context.Background(), "/", nil); err == nil {
		t.Fatal("exptected error")
	}
}

func TestResponseErrorMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&ErrorResponse{Message: "test-error"})
	}))
	defer ts.Close()

	if _, err := New(ts.URL, "").get(context.Background(), "/", nil); err == nil || err.Error() != "test-error" {
		t.Fatal("exptected error")
	}
}

func TestPostMarshalError(t *testing.T) {
	if _, err := New(noopServer.URL, "").post(context.Background(), "/", nil, func() {}); err == nil {
		t.Fatal("exptected json marshal error")
	}
}

func TestDoInvalidMethod(t *testing.T) {
	if _, err := New("", "").do(context.Background(), "/", "/", nil, nil, nil); err == nil {
		t.Fatal("exptected invalid method error")
	}
}

func TestDoInvalidRequest(t *testing.T) {
	if _, err := New("", "").do(context.Background(), "noop", "/", nil, nil, nil); err == nil {
		t.Fatal("exptected invalid method error")
	}
}

func TestDoTimeoutRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	if _, err := New(timeoutServer.URL, "").do(ctx, "get", "/", nil, nil, nil); err == nil {
		t.Fatal("exptected i/o timeout error")
	} else if err != nil && !strings.Contains(err.Error(), "i/o timeout") {
		t.Fatalf("exptected i/o timeout error, but got %s", err)
	}
}
