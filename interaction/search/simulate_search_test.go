package search

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/signature"
)

func TestSimulateSearch(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if valid, err := signature.Verify("synthetic-secret", r.Header, body); err != nil || !valid {
			t.Error("search signature is invalid", err)
		}
		var interaction dto.Interaction
		if err := json.Unmarshal(body, &interaction); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || interaction.ApplicationID != "fixture-app" || interaction.Data == nil {
			t.Error("invalid simulated request")
		}
		if interaction.Data != nil {
			var query dto.SearchInputResolved
			if err := json.Unmarshal(interaction.Data.Resolved, &query); err != nil || query.Keyword != "hello" {
				t.Error("search keyword lost", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{}`)
	}))
	defer server.Close()
	result, err := SimulateSearch(&Config{AppID: "fixture-app", EndPoint: server.URL, Secret: "synthetic-secret"}, "hello")
	if err != nil || result == nil || !called {
		t.Fatalf("result=%+v err=%v called=%t", result, err, called)
	}
}
