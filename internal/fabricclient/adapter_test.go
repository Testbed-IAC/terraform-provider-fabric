package fabricclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapi "github.com/Testbed-IAC/fabric-orchestrator-go-client"
)

type countingTokenSource struct {
	count  int
	token  string
	claims *FabricClaims
}

func (c *countingTokenSource) IDToken(context.Context) (string, error) {
	c.count++
	return c.token, nil
}

func (c *countingTokenSource) ProjectID() string {
	return c.claims.ProjectID()
}

func (c *countingTokenSource) Claims() *FabricClaims {
	return c.claims
}

func TestFabric_FabricClient_AdapterAuthCtx_UsesTokenSourceEachCall(t *testing.T) {
	t.Parallel()
	ts := &countingTokenSource{token: "token-1", claims: &FabricClaims{}}
	adapter := New("https://orchestrator.example", ts)
	for i := 0; i < 2; i++ {
		if _, err := adapter.authCtx(context.Background()); err != nil {
			t.Fatalf("authCtx returned error: %v", err)
		}
	}
	if ts.count != 2 {
		t.Fatalf("token source calls = %d, want 2", ts.count)
	}
}

func TestFabric_FabricClient_GetResourcesQuery(t *testing.T) {
	t.Parallel()

	seen := make(chan queryRecord, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- queryRecord{path: r.URL.Path, query: r.URL.RawQuery}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":[{"model":"<graphml/>"}]}`))
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	adapter := New(server.URL, &countingTokenSource{token: "token-1", claims: &FabricClaims{}})
	model, err := adapter.GetResources(context.Background(), ResourcesQuery{
		Level:        2,
		ForceRefresh: true,
		StartDate:    "2026-05-30 19:04:54 +00:00",
		EndDate:      "2026-05-31 19:04:54 +00:00",
		Includes:     "RENC,UKY",
		Excludes:     "STAR",
	})
	if err != nil {
		t.Fatalf("GetResources: %v", err)
	}
	if model != "<graphml/>" {
		t.Fatalf("model = %q, want <graphml/>", model)
	}
	got := <-seen
	if got.path != "/resources" {
		t.Fatalf("path = %q, want /resources", got.path)
	}
	assertQueryContains(t, got.query, "level=2")
	assertQueryContains(t, got.query, "force_refresh=true")
	assertQueryContains(t, got.query, "start_date=2026-05-30+19%3A04%3A54+%2B00%3A00")
	assertQueryContains(t, got.query, "end_date=2026-05-31+19%3A04%3A54+%2B00%3A00")
	assertQueryContains(t, got.query, "includes=RENC%2CUKY")
	assertQueryContains(t, got.query, "excludes=STAR")
}

func TestFabric_FabricClient_GetPortalResourcesQuery(t *testing.T) {
	t.Parallel()

	seen := make(chan queryRecord, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- queryRecord{path: r.URL.Path, query: r.URL.RawQuery}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":[{"model":"<portal/>"}]}`))
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	adapter := New(server.URL, &countingTokenSource{token: "token-1", claims: &FabricClaims{}})
	model, err := adapter.GetPortalResources(context.Background(), ResourcesQuery{
		Level:       1,
		GraphFormat: "GRAPHML",
	})
	if err != nil {
		t.Fatalf("GetPortalResources: %v", err)
	}
	if model != "<portal/>" {
		t.Fatalf("model = %q, want <portal/>", model)
	}
	got := <-seen
	if got.path != "/portalresources" {
		t.Fatalf("path = %q, want /portalresources", got.path)
	}
	assertQueryContains(t, got.query, "graph_format=GRAPHML")
}

func TestFabric_FabricClient_ConvertSlivers(t *testing.T) {
	t.Parallel()
	sliver := openapi.NewSliver("graph-node-1", "slice-1", "sliver-1")
	sliver.SetSliverType("NodeSliver")
	sliver.SetState("Active")
	sliver.SetPendingState("None")
	sliver.SetJoinState("Joined")
	sliver.Sliver = map[string]interface{}{"management_ip": "192.0.2.10"}

	got := convertSlivers([]openapi.Sliver{*sliver})
	if len(got) != 1 {
		t.Fatalf("slivers = %d, want 1", len(got))
	}
	if got[0].SliverID != "sliver-1" || got[0].GraphNodeID != "graph-node-1" || got[0].SliverType != "NodeSliver" {
		t.Fatalf("sliver identity = %+v, want decoded identity", got[0])
	}
	if got[0].State != "Active" || got[0].PendingState != "None" || got[0].JoinState != "Joined" {
		t.Fatalf("sliver state = %+v, want active/none/joined", got[0])
	}
	if got[0].ManagementIP != "192.0.2.10" {
		t.Fatalf("management_ip = %q, want 192.0.2.10", got[0].ManagementIP)
	}
}

func TestFabric_FabricClient_CreatePOA(t *testing.T) {
	t.Parallel()

	seen := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/poas/create/sliver-1" {
			t.Errorf("path = %q, want /poas/create/sliver-1", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		seen <- body
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":[{"poa_id":"poa-1","operation":"addkey","state":"Success","sliver_id":"sliver-1","slice_id":"slice-1","info":{"result":"ok"}}]}`))
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	adapter := New(server.URL, &countingTokenSource{token: "token-1", claims: &FabricClaims{}})
	poa, err := adapter.CreatePOA(context.Background(), "sliver-1", POARequest{
		Operation:  "addkey",
		VCPUCPUMap: []POAVCPUCPU{{VCPU: "0", CPU: "2"}},
		NodeSet:    []string{"node-1"},
		BDF:        []string{"0000:00:00.0"},
		Keys:       []POAKey{{Key: "ssh-ed25519 AAAA", Comment: "test"}},
	})
	if err != nil {
		t.Fatalf("CreatePOA: %v", err)
	}
	if poa.POAID != "poa-1" || poa.State != "Success" || poa.InfoJSON != `{"result":"ok"}` {
		t.Fatalf("poa = %+v, want mapped success response", poa)
	}
	body := <-seen
	if body["operation"] != "addkey" {
		t.Fatalf("operation = %v, want addkey", body["operation"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", body["data"])
	}
	if jsonListLen(t, data, "vcpu_cpu_map") != 1 || jsonListLen(t, data, "node_set") != 1 || jsonListLen(t, data, "bdf") != 1 || jsonListLen(t, data, "keys") != 1 {
		t.Fatalf("data = %#v, want all POA data collections", data)
	}
}

func TestFabric_FabricClient_GetPOA(t *testing.T) {
	t.Parallel()

	seen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":[{"poa_id":"poa-1","operation":"reboot","state":"Failed","error":"no host"}]}`))
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	adapter := New(server.URL, &countingTokenSource{token: "token-1", claims: &FabricClaims{}})
	poa, err := adapter.GetPOA(context.Background(), "poa-1")
	if err != nil {
		t.Fatalf("GetPOA: %v", err)
	}
	if <-seen != "/poas/poa-1" {
		t.Fatalf("unexpected POA path")
	}
	if poa.POAID != "poa-1" || poa.State != "Failed" || poa.Error != "no host" {
		t.Fatalf("poa = %+v, want failed response", poa)
	}
}

func TestFabric_FabricClient_GetMetricsOverview(t *testing.T) {
	t.Parallel()

	seen := make(chan queryRecord, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- queryRecord{path: r.URL.Path, query: r.URL.RawQuery}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"results":[{"project":"p1","nodes":2}]}`))
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	adapter := New(server.URL, &countingTokenSource{token: "token-1", claims: &FabricClaims{}})
	results, err := adapter.GetMetricsOverview(context.Background(), MetricsQuery{ExcludedProjects: []string{"p2", "p3"}})
	if err != nil {
		t.Fatalf("GetMetricsOverview: %v", err)
	}
	if results != `[{"nodes":2,"project":"p1"}]` {
		t.Fatalf("results = %q, want JSON array", results)
	}
	got := <-seen
	if got.path != "/metrics/overview" {
		t.Fatalf("path = %q, want /metrics/overview", got.path)
	}
	assertQueryContains(t, got.query, "excluded_projects=p2")
	assertQueryContains(t, got.query, "excluded_projects=p3")
}

func jsonListLen(t *testing.T, values map[string]any, key string) int {
	t.Helper()
	items, ok := values[key].([]any)
	if !ok {
		t.Fatalf("%s = %#v, want list", key, values[key])
	}
	return len(items)
}

type queryRecord struct {
	path  string
	query string
}

func assertQueryContains(t *testing.T, query, want string) {
	t.Helper()
	if !strings.Contains(query, want) {
		t.Fatalf("query = %q, want component %q", query, want)
	}
}
