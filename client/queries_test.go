package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkMethodAndPath(t, r, http.MethodPost, "/queries")
		json.NewEncoder(w).Encode(&QueriesResponse{})
	}))
	defer ts.Close()

	if _, err := New(ts.URL, "test-key").Queries(&QueriesRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestQueriesFail(t *testing.T) {
	if _, err := New(internalServerErrorServer.URL, "test-key").Queries(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestQueriesNoKey(t *testing.T) {
	if _, err := New("", "").Queries(nil); err != ErrNoAPIKey {
		t.Fatalf("expected error %s", ErrNoAPIKey)
	}
}

func TestQueriesInvalidJSON(t *testing.T) {
	if _, err := New(noopServer.URL, "test-key").Queries(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestQueriesRequest(t *testing.T) {
	query := [4]string{
		time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"0.0.0.0",
		"A",
		"alphasoc.net",
	}
	qr := NewQueriesRequest()
	qr.AddQuery(query)
	if len(qr.Data) != 1 {
		t.Fatalf("invalid number of queries - got %d; expected %d", len(qr.Data), 1)
	}
}
