package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFlipRoundTrip(t *testing.T) {
	flip := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/display" {
			http.NotFound(w, r)
			return
		}
		if v := r.URL.Query().Get("flip"); v != "" {
			if r.URL.Query().Get("token") != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid token"}`))
				return
			}
			flip = v == "1"
		}
		if flip {
			w.Write([]byte(`{"flip":true}`))
		} else {
			w.Write([]byte(`{"flip":false}`))
		}
	}))
	defer srv.Close()

	c := newAntennaSwitchClient()
	ctx := context.Background()

	if got, err := c.GetFlip(ctx, srv.URL); err != nil || got {
		t.Fatalf("GetFlip = %v, %v; want false, nil", got, err)
	}
	if got, err := c.SetFlip(ctx, srv.URL, "secret", true); err != nil || !got {
		t.Fatalf("SetFlip(true) = %v, %v; want true, nil", got, err)
	}
	if _, err := c.SetFlip(ctx, srv.URL, "wrong", false); err == nil || err.Error() != "invalid token" {
		t.Fatalf("SetFlip with wrong token err = %v; want invalid token", err)
	}
	if got, _ := c.GetFlip(ctx, srv.URL); !got {
		t.Fatal("flip should still be true after rejected request")
	}
}
