package fabricclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const refreshSkew = 5 * time.Minute

var ErrTokenExpired = errors.New("fabricclient: token expired")

type TokenSource interface {
	IDToken(ctx context.Context) (string, error)
	ProjectID() string
	Claims() *FabricClaims
}

type FabricProject struct {
	Name string   `json:"name"`
	UUID string   `json:"uuid"`
	Tags []string `json:"tags"`
}

type FabricClaims struct {
	Projects []FabricProject `json:"projects"`
	Exp      int64           `json:"exp"`
}

func (c *FabricClaims) Project() FabricProject {
	if c == nil || len(c.Projects) == 0 {
		return FabricProject{}
	}
	return c.Projects[0]
}

func (c *FabricClaims) ProjectID() string {
	return c.Project().UUID
}

func (c *FabricClaims) HasTag(tag string) bool {
	if c == nil || tag == "" {
		return false
	}
	for _, have := range c.Project().Tags {
		if have == tag {
			return true
		}
	}
	return false
}

func (c *FabricClaims) ExpiresAt() time.Time {
	if c == nil || c.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(c.Exp, 0)
}

type FabricTokenFile struct {
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	State        string `json:"state,omitempty"`
	TokenHash    string `json:"token_hash,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	CreatedFrom  string `json:"created_from,omitempty"`
}

type StaticToken struct {
	token  string
	claims *FabricClaims
}

func NewStaticToken(token string) (*StaticToken, error) {
	claims, err := ParseFabricJWT(token)
	if err != nil {
		return nil, fmt.Errorf("parsing static token: %w", err)
	}
	return &StaticToken{token: token, claims: claims}, nil
}

func (s *StaticToken) IDToken(ctx context.Context) (string, error) {
	if err := validateTokenTime(ctx, s.claims, false); err != nil {
		return "", err
	}
	return s.token, nil
}

func (s *StaticToken) ProjectID() string {
	return s.claims.ProjectID()
}

func (s *StaticToken) Claims() *FabricClaims {
	return s.claims
}

type FileToken struct {
	path       string
	credmgrURL string
	httpClient *http.Client
	mu         sync.Mutex
	tokenFile  FabricTokenFile
	claims     *FabricClaims
}

func NewFileToken(path, credmgrURL string, httpClient *http.Client) (*FileToken, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	tokenFile, err := ParseFabricTokenFile(path)
	if err != nil {
		return nil, err
	}
	claims, err := ParseFabricJWT(tokenFile.IDToken)
	if err != nil {
		return nil, fmt.Errorf("parsing token file id_token: %w", err)
	}
	return &FileToken{
		path:       path,
		credmgrURL: credmgrURL,
		httpClient: httpClient,
		tokenFile:  tokenFile,
		claims:     claims,
	}, nil
}

func (f *FileToken) IDToken(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := validateTokenTime(ctx, f.claims, true); err != nil && !errors.Is(err, ErrTokenExpired) {
		return "", err
	}
	if time.Until(f.claims.ExpiresAt()) > refreshSkew {
		return f.tokenFile.IDToken, nil
	}
	if f.tokenFile.RefreshToken == "" {
		return "", fmt.Errorf("refreshing token file %s: missing refresh_token", f.path)
	}
	refreshed, err := refreshToken(ctx, f.httpClient, f.credmgrURL, f.tokenFile.RefreshToken, f.claims.ProjectID(), f.claims.Project().Name, "all")
	if err != nil {
		return "", fmt.Errorf("refreshing token file %s: %w", f.path, err)
	}
	claims, err := ParseFabricJWT(refreshed.IDToken)
	if err != nil {
		return "", fmt.Errorf("parsing refreshed id_token: %w", err)
	}
	body, err := json.MarshalIndent(refreshed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling refreshed token file: %w", err)
	}
	if err := os.WriteFile(f.path, append(body, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("writing refreshed token file %s: %w", f.path, err)
	}
	f.tokenFile = refreshed
	f.claims = claims
	return f.tokenFile.IDToken, nil
}

func (f *FileToken) ProjectID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.claims.ProjectID()
}

func (f *FileToken) Claims() *FabricClaims {
	f.mu.Lock()
	defer f.mu.Unlock()
	claimsCopy := *f.claims
	claimsCopy.Projects = append([]FabricProject(nil), f.claims.Projects...)
	for i := range claimsCopy.Projects {
		claimsCopy.Projects[i].Tags = append([]string(nil), f.claims.Projects[i].Tags...)
	}
	return &claimsCopy
}

func ParseFabricTokenFile(path string) (FabricTokenFile, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return FabricTokenFile{}, fmt.Errorf("reading token file %s: %w", path, err)
	}
	var out FabricTokenFile
	if err := json.Unmarshal(body, &out); err != nil {
		return FabricTokenFile{}, fmt.Errorf("parsing token file %s: %w", path, err)
	}
	if out.IDToken == "" {
		return FabricTokenFile{}, fmt.Errorf("parsing token file %s: missing id_token", path)
	}
	return out, nil
}

func ParseFabricJWT(token string) (*FabricClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("jwt: malformed token: expected 3 dot-separated segments, got %d", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("jwt: malformed token: decoding payload: %w", err)
		}
	}
	var claims FabricClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, fmt.Errorf("jwt: malformed token: parsing payload: %w", err)
	}
	return &claims, nil
}

func validateTokenTime(ctx context.Context, claims *FabricClaims, refreshable bool) error {
	expiresAt := claims.ExpiresAt()
	if expiresAt.IsZero() {
		return nil
	}
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		if refreshable {
			return ErrTokenExpired
		}
		return fmt.Errorf("%w: request a fresh token from https://portal.fabric-testbed.net/experiments#tokens", ErrTokenExpired)
	}
	if remaining < 15*time.Minute {
		tflog.Warn(ctx, "FABRIC token expires soon", map[string]any{"expires_at": expiresAt.Format(time.RFC3339)})
	}
	return nil
}

func refreshToken(ctx context.Context, client *http.Client, credmgrURL, refreshToken, projectID, projectName, scope string) (FabricTokenFile, error) {
	endpoint, err := refreshEndpoint(credmgrURL)
	if err != nil {
		return FabricTokenFile{}, err
	}
	body := map[string]string{
		"refresh_token": refreshToken,
		"scope":         scope,
	}
	if projectID != "" {
		body["project_id"] = projectID
	} else if projectName != "" {
		body["project_name"] = projectName
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return FabricTokenFile{}, fmt.Errorf("marshalling refresh request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return FabricTokenFile{}, fmt.Errorf("building refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return FabricTokenFile{}, fmt.Errorf("calling credmgr refresh: %w", err)
	}
	defer func() {
		// The response body has already been fully decoded or the status code
		// returned; close errors do not change the refresh result.
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FabricTokenFile{}, fmt.Errorf("credmgr refresh returned HTTP %d", resp.StatusCode)
	}
	var refreshed FabricTokenFile
	if err := json.NewDecoder(resp.Body).Decode(&refreshed); err != nil {
		return FabricTokenFile{}, fmt.Errorf("decoding credmgr refresh response: %w", err)
	}
	if refreshed.IDToken == "" {
		return FabricTokenFile{}, errors.New("credmgr refresh response missing id_token")
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = refreshToken
	}
	return refreshed, nil
}

func refreshEndpoint(rawURL string) (string, error) {
	if rawURL == "" {
		rawURL = "cm.fabric-testbed.net"
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing credmgr_url: %w", err)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/credmgr/tokens/refresh"
	return parsed.String(), nil
}
