package tests

// SDKImportPath is the Go import path for the Kion SDK's generated client
// package. Every file in this provider that references generated types uses
// the alias `generated`, so flipping this constant on a release branch is
// enough to redirect kgen-emitted test files to a different SDK sub-package.
//
// Per release branch, this is pinned to the matching SDK sub-package:
//
//	main              → kion-sdk-go/generated/v3_15 (newest supported)
//	release-3.15.x    → kion-sdk-go/generated/v3_15
//	release-3.14.x    → kion-sdk-go/generated/v3_14
//	release-3.13.x    → kion-sdk-go/generated/v3_13
//	release-3.12.x    → kion-sdk-go/generated/v3_12
//
// NOTE: hand-written imports elsewhere in the provider still need their
// import lines updated on each branch (bulk sed); this constant only
// affects what kgen WRITES when scaffolding new test files.
const SDKImportPath = "github.com/kionsoftware/kion-sdk-go/generated/v3_15"
