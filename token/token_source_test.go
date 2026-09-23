package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTokenLifecycle(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body qqBotTokenReq
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.URL.Path != "/app/getAppAccessToken" || body.AppID != "app" || body.ClientSecret != "fixture-secret" {
			t.Errorf("token request=%s %s %+v", r.Method, r.URL.Path, body)
		}
		n := requests.Add(1)
		var expiry interface{} = "7200"
		if n > 1 {
			expiry = 7200
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": fmt.Sprintf("token-%d", n), "expires_in": expiry})
	}))
	defer server.Close()
	source := NewQQBotTokenSource(&QQBotCredentials{AppID: "app", AppSecret: "fixture-secret"}, WithEndpoint(server.URL+"/app/getAppAccessToken"))
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			value, err := source.Token()
			if err != nil {
				t.Error(err)
				return
			}
			if value.AccessToken != "token-1" || value.TokenType != "QQBot" || value.ExpiresIn != 7200 {
				t.Errorf("cached token=%+v", value)
			}
		}()
	}
	workers.Wait()
	if got := requests.Load(); got != 1 {
		t.Fatalf("token acquisition count=%d", got)
	}
	if !source.Invalidate("token-1") {
		t.Fatal("token invalidation failed")
	}
	fresh, err := source.TokenContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fresh.AccessToken != "token-2" || fresh.ExpiresIn != 7200 || !fresh.Valid() {
		t.Fatalf("refreshed token=%+v", fresh)
	}
}

type refreshTestSource func() (*oauth2.Token, error)

func (f refreshTestSource) Token() (*oauth2.Token, error) { return f() }

func TestRefreshSchedule(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := time.Now()
	samples := make(chan time.Time, 2)
	var calls atomic.Int32
	source := refreshTestSource(func() (*oauth2.Token, error) {
		switch calls.Add(1) {
		case 1:
			return &oauth2.Token{AccessToken: "cached", ExpiresIn: 7200, Expiry: started.Add(10 * time.Second)}, nil
		case 2:
			samples <- time.Now()
			return nil, errors.New("fixture refresh failure")
		default:
			samples <- time.Now()
			return &oauth2.Token{AccessToken: "fresh", ExpiresIn: 7200, Expiry: time.Now().Add(2 * time.Hour)}, nil
		}
	})
	if err := StartRefreshAccessToken(ctx, source); err != nil {
		t.Fatal(err)
	}
	previous := started
	for i := 0; i < 2; i++ {
		select {
		case when := <-samples:
			elapsed := when.Sub(previous)
			minimum := 400 * time.Millisecond
			if i == 1 {
				minimum = 900 * time.Millisecond
			}
			if elapsed < minimum || elapsed > 3*time.Second {
				t.Fatalf("refresh step %d elapsed=%v", i, elapsed)
			}
			previous = when
		case <-time.After(5 * time.Second):
			t.Fatalf("refresh step %d timed out", i)
		}
	}
}
