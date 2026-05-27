package provider

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// makeJWT builds a syntactically valid (but unsigned) JWT carrying the given
// projects claim, for tests of decodeJWTPayload / jwtProjectTags only.
func makeJWT(t *testing.T, projects []jwtProject) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(jwtPayload{Projects: projects})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("signature"))
	return header + "." + body + "." + sig
}

func TestFabric_JWT_DecodePayload(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		token       string
		wantErr     bool
		wantErrSent error
		wantProj    int
	}{
		{name: "well-formed", token: makeJWT(t, []jwtProject{{Name: "p1", UUID: "u1", Tags: []string{"A"}}}), wantProj: 1},
		{name: "empty projects", token: makeJWT(t, nil), wantProj: 0},
		{name: "malformed segments", token: "abc.def", wantErr: true, wantErrSent: errJWTMalformed},
		{name: "garbage payload", token: "abc.@@@.ghi", wantErr: true, wantErrSent: errJWTMalformed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeJWTPayload(tc.token)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("decodeJWTPayload err = nil, want error")
				}
				if tc.wantErrSent != nil && !errors.Is(err, tc.wantErrSent) {
					t.Fatalf("err = %v, want errors.Is(%v)", err, tc.wantErrSent)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeJWTPayload err = %v", err)
			}
			if len(got.Projects) != tc.wantProj {
				t.Fatalf("projects = %d, want %d", len(got.Projects), tc.wantProj)
			}
		})
	}
}

func TestFabric_JWT_ProjectTags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		projects  []jwtProject
		projectID string
		wantOK    bool
		wantTags  []string
	}{
		{
			name: "match returns tags",
			projects: []jwtProject{
				{UUID: "u1", Tags: []string{"Slice.Multisite", "VM.NoLimitDisk"}},
				{UUID: "u2", Tags: []string{"VM.NoLimitCPU"}},
			},
			projectID: "u1",
			wantOK:    true,
			wantTags:  []string{"Slice.Multisite", "VM.NoLimitDisk"},
		},
		{
			name: "no match returns not ok",
			projects: []jwtProject{
				{UUID: "u1", Tags: []string{"A"}},
			},
			projectID: "nope",
			wantOK:    false,
		},
		{
			name:      "no projects returns not ok",
			projectID: "u1",
			wantOK:    false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := makeJWT(t, tc.projects)
			gotTags, ok, err := jwtProjectTags(token, tc.projectID)
			if err != nil {
				t.Fatalf("jwtProjectTags err = %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if strings.Join(gotTags, ",") != strings.Join(tc.wantTags, ",") {
				t.Fatalf("tags = %v, want %v", gotTags, tc.wantTags)
			}
		})
	}
}

func TestFabric_JWT_ProjectTags_MalformedToken(t *testing.T) {
	t.Parallel()
	if _, _, err := jwtProjectTags("not-a-jwt", "anything"); err == nil {
		t.Fatal("expected error on malformed token")
	}
}
