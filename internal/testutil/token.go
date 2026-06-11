package testutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

// TokenProject is one project entry in a minted token.
type TokenProject struct {
	UUID string
	Name string
	Tags []string
}

// FullToken mints an HS256 JWT for TestProject carrying every capability tag
// (AllTags), exp = now+1h. This is the happy-path default — it never trips the
// provider's plan-time tag checks.
func FullToken() string {
	return TokenWithTags(AllTags...)
}

// TokenWithTags mints an HS256 JWT for TestProject with exactly the given tags.
// Pass no tags for an empty-tag token.
func TokenWithTags(tags ...string) string {
	if tags == nil {
		tags = []string{}
	}
	return TokenWithProjects([]TokenProject{{
		UUID: TestProjectUUID,
		Name: TestProjectName,
		Tags: tags,
	}})
}

// TokenWithProjects mints an HS256 JWT with an arbitrary project list. An empty slice
// produces "projects": [] (for the no-project configure-error scenario).
func TokenWithProjects(projects []TokenProject) string {
	return mintToken(projects, time.Now().Add(time.Hour))
}

// ExpiredToken mints a fully-privileged token whose exp is in the past. The provider
// rejects it client-side (auth.validateTokenTime) before any HTTP call — testmode
// never sees it (ACCEPTANCE_TEST_PLAN.md C3).
func ExpiredToken() string {
	return mintToken([]TokenProject{{
		UUID: TestProjectUUID,
		Name: TestProjectName,
		Tags: AllTags,
	}}, time.Now().Add(-time.Hour))
}

// mintToken builds header.payload.signature with HS256 over TestJWTSecret. The payload
// includes sub/email/uuid for parity with the testmode README (the orchestrator reads
// them); the provider only reads projects[] and exp.
func mintToken(projects []TokenProject, exp time.Time) string {
	if projects == nil {
		projects = []TokenProject{}
	}
	projClaims := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		projClaims = append(projClaims, map[string]any{
			"uuid": p.UUID,
			"name": p.Name,
			"tags": tags,
		})
	}
	payload := map[string]any{
		"sub":      TestUserSub,
		"email":    TestUserEmail,
		"uuid":     TestUserUUID,
		"projects": projClaims,
		"iat":      time.Now().Unix(),
		"exp":      exp.Unix(),
	}
	header := map[string]any{"alg": "HS256", "typ": "JWT"}

	signingInput := encodeSegment(header) + "." + encodeSegment(payload)
	mac := hmac.New(sha256.New, []byte(TestJWTSecret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig
}

func encodeSegment(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// The inputs are fixed maps of strings/ints — marshalling cannot fail in
		// practice. Panic so a test author sees it immediately rather than minting a
		// malformed token.
		panic("testutil: encoding JWT segment: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
