package fabricclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testJWT(t *testing.T, exp time.Time, projects []FabricProject) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(FabricClaims{Exp: exp.Unix(), Projects: projects})
	if err != nil {
		t.Fatalf("marshal jwt payload: %v", err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	return header + "." + body + "." + sig
}

func TestFabric_FabricClient_ParseFabricJWT_ProjectClaims(t *testing.T) {
	t.Parallel()
	token := testJWT(t, time.Now().Add(time.Hour), []FabricProject{{
		Name: "Project One",
		UUID: "project-1",
		Tags: []string{"Slice.Multisite"},
	}})
	claims, err := ParseFabricJWT(token)
	if err != nil {
		t.Fatalf("ParseFabricJWT returned error: %v", err)
	}
	if claims.ProjectID() != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", claims.ProjectID())
	}
	if claims.Project().Name != "Project One" {
		t.Fatalf("Project().Name = %q, want Project One", claims.Project().Name)
	}
	if !claims.HasTag("Slice.Multisite") {
		t.Fatalf("HasTag(Slice.Multisite) = false")
	}
}

func TestFabric_FabricClient_StaticToken_ExpiredErrors(t *testing.T) {
	t.Parallel()
	ts, err := NewStaticToken(testJWT(t, time.Now().Add(-time.Minute), nil))
	if err != nil {
		t.Fatalf("NewStaticToken returned error: %v", err)
	}
	if _, err := ts.IDToken(context.Background()); err == nil {
		t.Fatal("IDToken returned nil error for expired token")
	}
}

func TestFabric_FabricClient_FileToken_RefreshesAndWritesFile(t *testing.T) {
	t.Parallel()
	var refreshCalls atomic.Int64
	newToken := testJWT(t, time.Now().Add(time.Hour), []FabricProject{{
		Name: "Project One",
		UUID: "project-1",
		Tags: []string{"VM.NoLimitDisk"},
	}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalls.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/credmgr/tokens/refresh") {
			t.Fatalf("path = %s, want /credmgr/tokens/refresh suffix", r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["refresh_token"] != "refresh-1" {
			t.Fatalf("refresh_token = %q, want refresh-1", body["refresh_token"])
		}
		if body["project_id"] != "project-1" {
			t.Fatalf("project_id = %q, want project-1", body["project_id"])
		}
		_ = json.NewEncoder(w).Encode(FabricTokenFile{
			IDToken:      newToken,
			RefreshToken: "refresh-2",
		})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "id_token.json")
	initial := FabricTokenFile{
		IDToken:      testJWT(t, time.Now().Add(time.Minute), []FabricProject{{Name: "Project One", UUID: "project-1"}}),
		RefreshToken: "refresh-1",
	}
	body, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal token file: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	ts, err := NewFileToken(path, server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewFileToken returned error: %v", err)
	}
	got, err := ts.IDToken(context.Background())
	if err != nil {
		t.Fatalf("IDToken returned error: %v", err)
	}
	if got != newToken {
		t.Fatalf("IDToken = %q, want refreshed token", got)
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("refresh calls = %d, want 1", refreshCalls.Load())
	}
	written, err := ParseFabricTokenFile(path)
	if err != nil {
		t.Fatalf("ParseFabricTokenFile returned error: %v", err)
	}
	if written.RefreshToken != "refresh-2" {
		t.Fatalf("written refresh token = %q, want refresh-2", written.RefreshToken)
	}
}

func TestFabric_FabricClient_FileToken_DoesNotRefreshFreshToken(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected refresh request")
	}))
	defer server.Close()
	token := testJWT(t, time.Now().Add(time.Hour), []FabricProject{{UUID: "project-1"}})
	path := filepath.Join(t.TempDir(), "id_token.json")
	body, err := json.Marshal(FabricTokenFile{IDToken: token, RefreshToken: "refresh-1"})
	if err != nil {
		t.Fatalf("marshal token file: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	ts, err := NewFileToken(path, server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewFileToken returned error: %v", err)
	}
	got, err := ts.IDToken(context.Background())
	if err != nil {
		t.Fatalf("IDToken returned error: %v", err)
	}
	if got != token {
		t.Fatalf("IDToken = %q, want original token", got)
	}
}
