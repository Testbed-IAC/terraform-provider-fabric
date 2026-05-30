package fabricclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	openapi "github.com/Testbed-IAC/fabric-orchestrator-go-client"
)

type Adapter struct {
	api *openapi.APIClient
	ts  TokenSource
}

func New(orchestratorURL string, ts TokenSource) *Adapter {
	cfg := openapi.NewConfiguration()
	if orchestratorURL != "" {
		cfg.Servers = openapi.ServerConfigurations{{URL: orchestratorURL}}
	}
	// Permanent FABRIC orchestrator workaround: some valid JSON responses are
	// returned with Content-Type: text/html. The generated OpenAPI client checks
	// Content-Type before unmarshalling and otherwise falls through to the
	// untyped "undefined response type" error. Keep this transport wrapper in
	// place until the orchestrator itself is fixed.
	cfg.HTTPClient = withContentTypeFix(cfg.HTTPClient)
	return &Adapter{api: openapi.NewAPIClient(cfg), ts: ts}
}

func (a *Adapter) authCtx(ctx context.Context) (context.Context, error) {
	token, err := a.ts.IDToken(ctx)
	if err != nil {
		return ctx, fmt.Errorf("getting FABRIC id token: %w", err)
	}
	return context.WithValue(ctx, openapi.ContextAccessToken, token), nil
}

func statusCodeOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func (a *Adapter) CreateSlice(ctx context.Context, name, graphML string, sshKeys []string, opts CreateOpts) ([]Sliver, error) {
	tflog.Info(ctx, "calling SlicesCreatesPost", map[string]any{
		"slice_name":    name,
		"graphml_bytes": len(graphML),
		"ssh_key_count": len(sshKeys),
		"lifetime":      opts.LifetimeHours,
	})

	body := openapi.NewSlicesPost(graphML, sshKeys)
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	req := a.api.SlicesAPI.SlicesCreatesPost(authCtx).Name(name).SlicesPost(*body)
	if opts.LifetimeHours > 0 {
		req = req.Lifetime(opts.LifetimeHours)
	}
	if opts.LeaseStartTime != "" {
		req = req.LeaseStartTime(opts.LeaseStartTime)
	}
	if opts.LeaseEndTime != "" {
		req = req.LeaseEndTime(opts.LeaseEndTime)
	}
	resp, httpResp, err := req.Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "SlicesCreatesPost failed", map[string]any{
			"slice_name":  name,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return nil, mapped
	}
	slivers := convertSlivers(resp.Data)
	tflog.Info(ctx, "SlicesCreatesPost succeeded", map[string]any{
		"slice_name":   name,
		"sliver_count": len(slivers),
	})
	return slivers, nil
}

func (a *Adapter) GetSlice(ctx context.Context, sliceID string) (*Slice, error) {
	tflog.Info(ctx, "calling SlicesSliceIdGet", map[string]any{"slice_id": sliceID})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	resp, httpResp, err := a.api.SlicesAPI.SlicesSliceIdGet(authCtx, sliceID).GraphFormat("GRAPHML").Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "SlicesSliceIdGet failed", map[string]any{
			"slice_id":    sliceID,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return nil, mapped
	}
	if resp == nil || len(resp.Data) == 0 {
		tflog.Info(ctx, "SlicesSliceIdGet returned no slice", map[string]any{"slice_id": sliceID})
		return nil, ErrNotFound
	}
	s := resp.Data[0]
	tflog.Info(ctx, "SlicesSliceIdGet succeeded", map[string]any{
		"slice_id": sliceID,
		"state":    s.GetState(),
	})
	return &Slice{
		SliceID:        s.GetSliceId(),
		Name:           s.GetName(),
		State:          s.GetState(),
		GraphID:        s.GetGraphId(),
		Model:          s.GetModel(),
		LeaseStartTime: s.GetLeaseStartTime(),
		LeaseEndTime:   s.GetLeaseEndTime(),
	}, nil
}

func (a *Adapter) ListSlices(ctx context.Context, name string, states []string) ([]Slice, error) {
	tflog.Info(ctx, "calling SlicesGet", map[string]any{
		"name_filter":  name,
		"state_filter": states,
	})
	out := []Slice{}
	var offset int32
	for {
		authCtx, err := a.authCtx(ctx)
		if err != nil {
			return nil, err
		}
		req := a.api.SlicesAPI.SlicesGet(authCtx).Limit(200).Offset(offset)
		if name != "" {
			req = req.Name(name).ExactMatch(true)
		}
		if len(states) > 0 {
			req = req.States(states)
		}
		page, httpResp, err := req.Execute()
		if err != nil {
			mapped := mapHTTPErr(httpResp, err)
			tflog.Error(ctx, "SlicesGet failed", map[string]any{
				"name_filter": name,
				"offset":      offset,
				"http_status": statusCodeOf(httpResp),
				"error":       mapped.Error(),
			})
			return nil, mapped
		}
		for _, s := range page.Data {
			out = append(out, Slice{SliceID: s.GetSliceId(), Name: s.GetName(), GraphID: s.GetGraphId(), State: s.GetState()})
		}
		if len(page.Data) < 200 {
			tflog.Info(ctx, "SlicesGet succeeded", map[string]any{
				"name_filter": name,
				"slice_count": len(out),
			})
			return out, nil
		}
		offset += 200
	}
}

func (a *Adapter) ModifySlice(ctx context.Context, sliceID, graphML string) ([]Sliver, error) {
	tflog.Info(ctx, "calling SlicesModifySliceIdPut", map[string]any{
		"slice_id":      sliceID,
		"graphml_bytes": len(graphML),
	})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	resp, httpResp, err := a.api.SlicesAPI.SlicesModifySliceIdPut(authCtx, sliceID).Body(graphML).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "SlicesModifySliceIdPut failed", map[string]any{
			"slice_id":    sliceID,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return nil, mapped
	}
	slivers := convertSlivers(resp.Data)
	tflog.Info(ctx, "SlicesModifySliceIdPut succeeded", map[string]any{
		"slice_id":     sliceID,
		"sliver_count": len(slivers),
	})
	return slivers, nil
}

func (a *Adapter) AcceptModify(ctx context.Context, sliceID string) (*Slice, error) {
	tflog.Info(ctx, "calling SlicesModifySliceIdAcceptPost", map[string]any{"slice_id": sliceID})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	resp, httpResp, err := a.api.SlicesAPI.SlicesModifySliceIdAcceptPost(authCtx, sliceID).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "SlicesModifySliceIdAcceptPost failed", map[string]any{
			"slice_id":    sliceID,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return nil, mapped
	}
	if resp == nil || len(resp.Data) == 0 {
		return nil, ErrNotFound
	}
	s := resp.Data[0]
	tflog.Info(ctx, "SlicesModifySliceIdAcceptPost succeeded", map[string]any{
		"slice_id": sliceID,
		"state":    s.GetState(),
	})
	return &Slice{SliceID: s.GetSliceId(), Name: s.GetName(), State: s.GetState(), GraphID: s.GetGraphId(), Model: s.GetModel()}, nil
}

func (a *Adapter) RenewSlice(ctx context.Context, sliceID, leaseEndTime string) error {
	tflog.Info(ctx, "calling SlicesRenewSliceIdPost", map[string]any{
		"slice_id":       sliceID,
		"lease_end_time": leaseEndTime,
	})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return err
	}
	_, httpResp, err := a.api.SlicesAPI.SlicesRenewSliceIdPost(authCtx, sliceID).LeaseEndTime(leaseEndTime).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "SlicesRenewSliceIdPost failed", map[string]any{
			"slice_id":    sliceID,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return mapped
	}
	tflog.Info(ctx, "SlicesRenewSliceIdPost succeeded", map[string]any{"slice_id": sliceID})
	return nil
}

func (a *Adapter) DeleteSlice(ctx context.Context, sliceID string) error {
	tflog.Info(ctx, "calling SlicesDeleteSliceIdDelete", map[string]any{"slice_id": sliceID})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return err
	}
	_, httpResp, err := a.api.SlicesAPI.SlicesDeleteSliceIdDelete(authCtx, sliceID).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "SlicesDeleteSliceIdDelete failed", map[string]any{
			"slice_id":    sliceID,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return mapped
	}
	tflog.Info(ctx, "SlicesDeleteSliceIdDelete succeeded", map[string]any{"slice_id": sliceID})
	return nil
}

func (a *Adapter) GetSlivers(ctx context.Context, sliceID string) ([]Sliver, error) {
	tflog.Info(ctx, "calling SliversGet", map[string]any{"slice_id": sliceID})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	resp, httpResp, err := a.api.SliversAPI.SliversGet(authCtx).SliceId(sliceID).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "SliversGet failed", map[string]any{
			"slice_id":    sliceID,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return nil, mapped
	}
	slivers := convertSlivers(resp.Data)
	tflog.Info(ctx, "SliversGet succeeded", map[string]any{
		"slice_id":     sliceID,
		"sliver_count": len(slivers),
	})
	return slivers, nil
}

func (a *Adapter) GetResources(ctx context.Context, level int32, forceRefresh bool) (string, error) {
	tflog.Info(ctx, "calling ResourcesGet", map[string]any{
		"level":         level,
		"force_refresh": forceRefresh,
	})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return "", err
	}
	resp, httpResp, err := a.api.ResourcesAPI.ResourcesGet(authCtx).Level(level).ForceRefresh(forceRefresh).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "ResourcesGet failed", map[string]any{
			"level":       level,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return "", mapped
	}
	if resp == nil || len(resp.Data) == 0 {
		tflog.Info(ctx, "ResourcesGet returned no data", map[string]any{"level": level})
		return "", nil
	}
	model := resp.Data[0].GetModel()
	tflog.Info(ctx, "ResourcesGet succeeded", map[string]any{
		"level":       level,
		"model_bytes": len(model),
	})
	return model, nil
}

func convertSlivers(in []openapi.Sliver) []Sliver {
	out := make([]Sliver, 0, len(in))
	for _, s := range in {
		out = append(out, Sliver{
			SliceID:     s.GetSliceId(),
			SliverID:    s.GetSliverId(),
			GraphNodeID: s.GetGraphNodeId(),
			SliverType:  s.GetSliverType(),
			State:       s.GetState(),
			Notice:      s.GetNotice(),
		})
	}
	return out
}
