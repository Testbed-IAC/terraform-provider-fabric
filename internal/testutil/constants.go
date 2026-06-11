package testutil

import "github.com/Testbed-IAC/fabric-go-fim/pkg/permission"

// Hardcoded identity used by every acceptance/permission test. Nothing here is read
// from the environment — the test binary is its own IdP. testmode does not verify the
// JWT signature, does not enforce exp, and accepts any project UUID as-is, so these
// values only need to be internally consistent.
//
// See ACCEPTANCE_TEST_PLAN.md §0.1 (C2/C3/C4) and the testmode README.
const (
	TestUserSub     = "test-user-sub"
	TestUserEmail   = "tester@example.com"
	TestUserUUID    = "00000000-0000-0000-0000-000000000001"
	TestProjectUUID = "11111111-1111-1111-1111-111111111111"
	TestProjectName = "TestProject"
	TestSite        = "RENC"
	TestSiteUKY     = "UKY" // only placeable when the §4f second site AM is running
	// TestJWTSecret signs minted tokens with HS256. The signature is never verified
	// (provider decodes claims only; testmode has oauth.verify-sig=False), so the value
	// is arbitrary — it matches the testmode README for parity.
	TestJWTSecret = "irrelevant-secret"

	// DummySSHKey is accepted verbatim by testmode (substrate is mocked; the key is
	// never used to reach a host). A syntactically valid public key avoids any
	// provider-side validation surprises.
	DummySSHKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestTestTestTestTestTestTestTestTestTest tester@example.com"
)

// AllTags is the full provider capability vocabulary (source of truth:
// fabric-go-fim/pkg/permission/tags.go) plus the orchestrator-side operation tags.
// The latter are inert in testmode (PDP disabled — see testmode-no-tag-enforcement)
// but are included so a FullToken never blocks the happy path and stays realistic if
// PDP is ever enabled. Used as the default for happy-path tests.
var AllTags = []string{
	permission.TagSliceNoLimitLifetime,
	permission.TagVMNoLimit,
	permission.TagVMNoLimitCPU,
	permission.TagVMNoLimitRAM,
	permission.TagVMNoLimitDisk,
	permission.TagSliceMultisite,
	permission.TagComponentGPU,
	permission.TagComponentFPGA,
	permission.TagComponentNVME,
	permission.TagComponentStorage,
	permission.TagComponentSmartNICConnectX5,
	permission.TagComponentSmartNICConnectX6,
	permission.TagComponentSmartNICConnectX7100,
	permission.TagComponentSmartNICConnectX7400,
	permission.TagComponentSmartNICBlueField2ConnectX6,
	permission.TagNetNoLimitBW,
	permission.TagNetFABNetv4Ext,
	permission.TagNetFABNetv6Ext,
	permission.TagNetPortMirroring,
	// Orchestrator-side operation tags (inert in testmode, PDP disabled):
	"Slice.Create", "Slice.Modify", "Slice.Delete",
}

// TagsExcept returns AllTags with the given tags removed. Used by permission tests to
// build a token that is fully privileged except for the one capability under test.
func TagsExcept(remove ...string) []string {
	drop := make(map[string]struct{}, len(remove))
	for _, t := range remove {
		drop[t] = struct{}{}
	}
	out := make([]string, 0, len(AllTags))
	for _, t := range AllTags {
		if _, skip := drop[t]; skip {
			continue
		}
		out = append(out, t)
	}
	return out
}
