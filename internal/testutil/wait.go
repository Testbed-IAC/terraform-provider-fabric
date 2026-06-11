package testutil

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
)

// WaitForOrchestrator polls GET {url}/version until it returns 200 or the timeout
// elapses. The content-type bug is irrelevant for a status-code check.
func WaitForOrchestrator(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	endpoint := url + "/version"
	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("building version request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		lastErr = fmt.Errorf("GET /version returned HTTP %d", resp.StatusCode)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("orchestrator not ready at %s after %s: %w", endpoint, timeout, lastErr)
}

// WaitForResources polls the orchestrator resources model until the broker CBM
// advertises at least minWorkers compute (Server) nodes, or the timeout elapses. This
// gates on the one-shot `claim` service having merged the AM delegations into the CBM —
// until then /resources has no placeable compute even though /version answers.
//
// The check counts "Server" NetworkNode entries in the raw BQM GraphML rather than
// decoding it: the live orchestrator's broker query model uses a different GraphML
// d-key layout than catalog.DecodeAdvertised (which targets the advertised-resources
// data-source schema), so a structural decode reports zero. minWorkers is 1 for the
// default single-site stack (RENCI-ad advertises 3 RENC workers) and 4 when the §4f UKY
// site AM is enabled (3 RENC + 3 UKY = 6, cleanly above the single-site count).
func WaitForResources(url string, minWorkers int, timeout time.Duration) error {
	client, err := Client()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		// Level 2 expands per-worker NetworkNodes (level 1 collapses them), so the
		// Server count reflects both sites when the §4f UKY AM is running.
		model, err := client.GetResources(ctx, fabricclient.ResourcesQuery{Level: 2, ForceRefresh: true})
		cancel()
		if err != nil {
			lastErr = err
			time.Sleep(3 * time.Second)
			continue
		}
		if n := advertisedWorkerCount(model); n >= minWorkers {
			return nil
		} else {
			lastErr = fmt.Errorf("broker CBM advertises %d compute worker(s), want >= %d", n, minWorkers)
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("resources not populated at %s after %s (claim service may not have run): %w", url, timeout, lastErr)
}

// advertisedWorkerCount counts compute worker nodes in the BQM by the number of
// NetworkNode entries advertised as Type "Server".
func advertisedWorkerCount(model string) int {
	if model == "" {
		return 0
	}
	return strings.Count(model, ">Server<")
}
