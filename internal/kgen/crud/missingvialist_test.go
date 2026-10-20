package crud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every missing_via_list declared in private_endpoints.yaml must reach the
// generated read. Dropping it silently brings back the 500 that broke refresh
// of a resource deleted outside Terraform.
func TestMissingViaListReachesGeneratedRead(t *testing.T) {
	t.Parallel()
	eps, err := loadPrivateEndpoints(filepath.Join("..", "..", "..", "codegen", "private_endpoints.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for pkg, pe := range eps {
		if pe.MissingViaList == "" {
			continue
		}
		n++
		src, err := os.ReadFile(filepath.Join("..", "..", "service", pkg, pkg+".go"))
		if err != nil {
			t.Fatal(err)
		}
		want := `RawIDListed(ctx, "` + pe.MissingViaList + `", id)`
		if !strings.Contains(string(src), want) {
			t.Errorf("%s declares missing_via_list %s but its read does not consult it; run `make crud-force`", pkg, pe.MissingViaList)
		}
	}
	if n == 0 {
		t.Fatal("no resource declares missing_via_list; app_role should")
	}
}
