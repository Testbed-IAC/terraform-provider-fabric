package fabricclient

import (
	"context"
	"encoding/json"
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

func (a *Adapter) GetResources(ctx context.Context, query ResourcesQuery) (string, error) {
	tflog.Info(ctx, "calling ResourcesGet", map[string]any{
		"level":         query.Level,
		"force_refresh": query.ForceRefresh,
		"includes":      query.Includes,
		"excludes":      query.Excludes,
	})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return "", err
	}
	req := a.api.ResourcesAPI.ResourcesGet(authCtx).Level(query.Level).ForceRefresh(query.ForceRefresh)
	if query.StartDate != "" {
		req = req.StartDate(query.StartDate)
	}
	if query.EndDate != "" {
		req = req.EndDate(query.EndDate)
	}
	if query.Includes != "" {
		req = req.Includes(query.Includes)
	}
	if query.Excludes != "" {
		req = req.Excludes(query.Excludes)
	}
	resp, httpResp, err := req.Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "ResourcesGet failed", map[string]any{
			"level":       query.Level,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return "", mapped
	}
	if resp == nil || len(resp.Data) == 0 {
		tflog.Info(ctx, "ResourcesGet returned no data", map[string]any{"level": query.Level})
		return "", nil
	}
	model := resp.Data[0].GetModel()
	tflog.Info(ctx, "ResourcesGet succeeded", map[string]any{
		"level":       query.Level,
		"model_bytes": len(model),
	})
	return model, nil
}

func (a *Adapter) GetPortalResources(ctx context.Context, query ResourcesQuery) (string, error) {
	tflog.Info(ctx, "calling PortalresourcesGet", map[string]any{
		"level":         query.Level,
		"force_refresh": query.ForceRefresh,
		"graph_format":  query.GraphFormat,
		"includes":      query.Includes,
		"excludes":      query.Excludes,
	})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return "", err
	}
	req := a.api.ResourcesAPI.PortalresourcesGet(authCtx).GraphFormat(query.GraphFormat).Level(query.Level).ForceRefresh(query.ForceRefresh)
	if query.StartDate != "" {
		req = req.StartDate(query.StartDate)
	}
	if query.EndDate != "" {
		req = req.EndDate(query.EndDate)
	}
	if query.Includes != "" {
		req = req.Includes(query.Includes)
	}
	if query.Excludes != "" {
		req = req.Excludes(query.Excludes)
	}
	resp, httpResp, err := req.Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "PortalresourcesGet failed", map[string]any{
			"level":       query.Level,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return "", mapped
	}
	if resp == nil || len(resp.Data) == 0 {
		tflog.Info(ctx, "PortalresourcesGet returned no data", map[string]any{"level": query.Level})
		return "", nil
	}
	model := resp.Data[0].GetModel()
	tflog.Info(ctx, "PortalresourcesGet succeeded", map[string]any{
		"level":       query.Level,
		"model_bytes": len(model),
	})
	return model, nil
}

func (a *Adapter) CreatePOA(ctx context.Context, sliverID string, request POARequest) (*POA, error) {
	tflog.Info(ctx, "calling PoasCreateSliverIdPost", map[string]any{"sliver_id": sliverID, "operation": request.Operation})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	resp, httpResp, err := a.api.PoasAPI.PoasCreateSliverIdPost(authCtx, sliverID).PoaPost(openapiPOARequest(request)).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "PoasCreateSliverIdPost failed", map[string]any{
			"sliver_id":   sliverID,
			"operation":   request.Operation,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return nil, mapped
	}
	poa, err := convertPOA(resp)
	if err != nil {
		return nil, fmt.Errorf("converting poa response: %w", err)
	}
	tflog.Info(ctx, "PoasCreateSliverIdPost succeeded", map[string]any{"sliver_id": sliverID, "operation": request.Operation, "poa_id": poa.POAID})
	return poa, nil
}

func (a *Adapter) GetPOA(ctx context.Context, poaID string) (*POA, error) {
	tflog.Info(ctx, "calling PoasPoaIdGet", map[string]any{"poa_id": poaID})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	resp, httpResp, err := a.api.PoasAPI.PoasPoaIdGet(authCtx, poaID).Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "PoasPoaIdGet failed", map[string]any{
			"poa_id":      poaID,
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return nil, mapped
	}
	poa, err := convertPOA(resp)
	if err != nil {
		return nil, fmt.Errorf("converting poa response: %w", err)
	}
	tflog.Info(ctx, "PoasPoaIdGet succeeded", map[string]any{"poa_id": poaID, "state": poa.State})
	return poa, nil
}

func (a *Adapter) GetMetricsOverview(ctx context.Context, query MetricsQuery) (string, error) {
	tflog.Info(ctx, "calling MetricsOverviewGet", map[string]any{"excluded_project_count": len(query.ExcludedProjects)})
	authCtx, err := a.authCtx(ctx)
	if err != nil {
		return "", err
	}
	req := a.api.MetricsAPI.MetricsOverviewGet(authCtx)
	if len(query.ExcludedProjects) > 0 {
		req = req.ExcludedProjects(query.ExcludedProjects)
	}
	resp, httpResp, err := req.Execute()
	if err != nil {
		mapped := mapHTTPErr(httpResp, err)
		tflog.Error(ctx, "MetricsOverviewGet failed", map[string]any{
			"http_status": statusCodeOf(httpResp),
			"error":       mapped.Error(),
		})
		return "", mapped
	}
	results, err := metricsResultsJSON(resp)
	if err != nil {
		return "", fmt.Errorf("converting metrics overview: %w", err)
	}
	tflog.Info(ctx, "MetricsOverviewGet succeeded", map[string]any{"results_bytes": len(results)})
	return results, nil
}

func convertSlivers(in []openapi.Sliver) []Sliver {
	out := make([]Sliver, 0, len(in))
	for _, s := range in {
		out = append(out, Sliver{
			SliceID:      s.GetSliceId(),
			SliverID:     s.GetSliverId(),
			GraphNodeID:  s.GetGraphNodeId(),
			SliverType:   s.GetSliverType(),
			State:        s.GetState(),
			PendingState: s.GetPendingState(),
			JoinState:    s.GetJoinState(),
			ManagementIP: managementIPFromSliverPayload(s.GetSliver()),
			Notice:       s.GetNotice(),
		})
	}
	return out
}

func openapiPOARequest(request POARequest) openapi.PoaPost {
	out := openapi.NewPoaPost(request.Operation)
	data := openapi.PoaPostData{}
	for _, mapping := range request.VCPUCPUMap {
		data.VcpuCpuMap = append(data.VcpuCpuMap, *openapi.NewPoaPostDataVcpuCpuMap(mapping.VCPU, mapping.CPU))
	}
	data.NodeSet = append([]string(nil), request.NodeSet...)
	data.Bdf = append([]string(nil), request.BDF...)
	for _, key := range request.Keys {
		data.Keys = append(data.Keys, *openapi.NewPoaPostDataKeys(key.Key, key.Comment))
	}
	if len(data.VcpuCpuMap) > 0 || len(data.NodeSet) > 0 || len(data.Bdf) > 0 || len(data.Keys) > 0 {
		out.SetData(data)
	}
	return *out
}

func convertPOA(in *openapi.Poa) (*POA, error) {
	if in == nil || len(in.GetData()) == 0 {
		return &POA{}, nil
	}
	data := in.GetData()[0]
	info, err := infoJSON(data.GetInfo())
	if err != nil {
		return nil, fmt.Errorf("encoding poa info: %w", err)
	}
	return &POA{
		POAID:     data.GetPoaId(),
		Operation: data.GetOperation(),
		State:     data.GetState(),
		SliverID:  data.GetSliverId(),
		SliceID:   data.GetSliceId(),
		Error:     data.GetError(),
		InfoJSON:  info,
	}, nil
}

func infoJSON(info map[string]interface{}) (string, error) {
	if len(info) == 0 {
		return "", nil
	}
	body, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("marshal info: %w", err)
	}
	return string(body), nil
}

func metricsResultsJSON(metrics *openapi.Metrics) (string, error) {
	results := []map[string]interface{}{}
	if metrics != nil {
		results = metrics.GetResults()
	}
	body, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("marshal metrics results: %w", err)
	}
	return string(body), nil
}

func managementIPFromSliverPayload(payload map[string]interface{}) string {
	for _, key := range []string{"management_ip", "managementIP", "mgmt_ip", "mgmtIP"} {
		value, ok := payload[key].(string)
		if ok {
			return value
		}
	}
	return ""
}
