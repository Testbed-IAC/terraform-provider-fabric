package provider

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// jwtProject mirrors the per-project entry inside a FABRIC JWT payload.
type jwtProject struct {
	Name string   `json:"name"`
	UUID string   `json:"uuid"`
	Tags []string `json:"tags"`
}

// jwtPayload mirrors the FABRIC JWT payload fields we care about.
type jwtPayload struct {
	Projects []jwtProject `json:"projects"`
	Exp      int64        `json:"exp"`
}

var errJWTMalformed = errors.New("jwt: malformed token")

// decodeJWTPayload decodes the payload (middle segment) of a JWT without
// verifying the signature. FABRIC tokens are issued by CILogon and the
// orchestrator is the authoritative verifier; the provider only needs the
// payload claims for client-side hints.
func decodeJWTPayload(token string) (*jwtPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: expected 3 dot-separated segments, got %d", errJWTMalformed, len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some issuers pad the payload — RawURLEncoding rejects padding,
		// URLEncoding accepts it.
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: payload base64 decode: %w", errJWTMalformed, err)
		}
	}
	var out jwtPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%w: payload json: %w", errJWTMalformed, err)
	}
	return &out, nil
}

// jwtProjectTags returns the tag set for the given project_id from the token's
// projects claim. Returns (nil, false) if the project_id is not present.
func jwtProjectTags(token, projectID string) ([]string, bool, error) {
	payload, err := decodeJWTPayload(token)
	if err != nil {
		return nil, false, err
	}
	for _, p := range payload.Projects {
		if p.UUID == projectID {
			return p.Tags, true, nil
		}
	}
	return nil, false, nil
}
