package acctest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Install capability, as opposed to the KION_ACC_* ids.
//
// A test that cannot run because the install lacks a prerequisite must skip
// naming what is missing, not fail. A missing id already does that; an install
// whose financial mode or cloud connections rule the resource out did not, and
// failed with an API error instead:
//
//	Bad Request: spend plans not available in budget mode
//	Internal Server Error: We do not have access to any Azure Billing Sources
//
// That defeats the skip/fail split the acctest workflow summary is built on --
// a mostly-skipped run against a minimal install must not read as a mostly-
// failing one, and real bugs must not hide in that noise.

// Capabilities describes what the target install can actually exercise.
type Capabilities struct {
	// BudgetMode is Kion's financial mode. Spend plans, and so any project
	// carrying a project_funding block, are unavailable when it is set.
	BudgetMode bool
}

// Cached together rather than as package-level vars so the error is plainly
// the fetch's result, not a sentinel other code compares against.
var installCaps struct {
	once sync.Once
	val  Capabilities
	err  error
}

// InstallCapabilities reads the target install's capabilities, once per run.
func InstallCapabilities() (Capabilities, error) {
	installCaps.once.Do(func() {
		installCaps.val, installCaps.err = fetchCapabilities(os.Getenv)
	})
	return installCaps.val, installCaps.err
}

func fetchCapabilities(getenv func(string) string) (_ Capabilities, err error) {
	var caps Capabilities

	base := strings.TrimRight(getenv("KION_API_URL"), "/")
	if base == "" {
		return caps, fmt.Errorf("KION_API_URL must be set")
	}
	apiPath := "/api"
	if v, ok := os.LookupEnv("KION_APIPATH"); ok {
		apiPath = v
	}
	token := getenv("KION_API_KEY")
	if token == "" {
		token = getenv("KION_AUTH_TOKEN")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	url := base + strings.TrimRight(apiPath, "/") + "/v3/app-config"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return caps, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return caps, fmt.Errorf("reading install capabilities from %s: %w", url, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return caps, fmt.Errorf("reading install capabilities from %s: HTTP %d", url, resp.StatusCode)
	}

	var body struct {
		Data struct {
			BudgetMode bool `json:"budget_mode"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return caps, fmt.Errorf("decoding install capabilities: %w", err)
	}

	caps.BudgetMode = body.Data.BudgetMode
	return caps, nil
}

// RequireSpendPlanMode skips when the install runs in budget mode.
//
// A failure to read the capability is fatal rather than a skip: an unreachable
// install should not quietly turn every affected test green.
func RequireSpendPlanMode(t *testing.T) {
	t.Helper()

	caps, err := InstallCapabilities()
	if err != nil {
		t.Fatalf("could not determine whether the install is in budget mode: %v", err)
	}
	if caps.BudgetMode {
		t.Skip("install is in budget mode: spend plans, and so project_funding, are unavailable")
	}
}

// RequireEnv skips when an install-specific id is unset, naming it and saying
// what it should hold.
func RequireEnv(t *testing.T, name, describes string) string {
	t.Helper()

	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s must be set to %s", name, describes)
	}
	return v
}
