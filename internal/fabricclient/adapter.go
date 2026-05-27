package fabricclient

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	openapi "github.com/Testbed-IAC/fabric-orchestrator-go-client"
)

type Adapter struct {
	api   *openapi.APIClient
	token string
}

func New(orchestratorURL, token string) *Adapter {
	cfg := openapi.NewConfiguration()
	if orchestratorURL != "" {
		cfg.Servers = openapi.ServerConfigurations{{URL: orchestratorURL}}
	}
	// FABRIC orchestrator returns Content-Type: text/html on some endpoints
	// even though the body is valid JSON. The generated client rejects that
	// with "undefined response type". Patch the header in transit so the
	// client can decode normally.
	cfg.HTTPClient = withContentTypeFix(cfg.HTTPClient)
	return &Adapter{api: openapi.NewAPIClient(cfg), token: token}
}

func (a *Adapter) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, openapi.ContextAccessToken, a.token)
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
	req := a.api.SlicesAPI.SlicesCreatesPost(a.authCtx(ctx)).Name(name).SlicesPost(*body)
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
	resp, httpResp, err := a.api.SlicesAPI.SlicesSliceIdGet(a.authCtx(ctx), sliceID).GraphFormat("GRAPHML").Execute()
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
		req := a.api.SlicesAPI.SlicesGet(a.authCtx(ctx)).Limit(200).Offset(offset)
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
	resp, httpResp, err := a.api.SlicesAPI.SlicesModifySliceIdPut(a.authCtx(ctx), sliceID).Body(graphML).Execute()
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
	resp, httpResp, err := a.api.SlicesAPI.SlicesModifySliceIdAcceptPost(a.authCtx(ctx), sliceID).Execute()
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
	_, httpResp, err := a.api.SlicesAPI.SlicesRenewSliceIdPost(a.authCtx(ctx), sliceID).LeaseEndTime(leaseEndTime).Execute()
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
	_, httpResp, err := a.api.SlicesAPI.SlicesDeleteSliceIdDelete(a.authCtx(ctx), sliceID).Execute()
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
	resp, httpResp, err := a.api.SliversAPI.SliversGet(a.authCtx(ctx)).SliceId(sliceID).Execute()
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
	resp, httpResp, err := a.api.ResourcesAPI.ResourcesGet(a.authCtx(ctx)).Level(level).ForceRefresh(forceRefresh).Execute()
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
