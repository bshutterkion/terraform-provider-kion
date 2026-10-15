package module

import (
	"strings"
	"testing"

	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// A computed-only sensitive attribute is an output, and Terraform refuses a
// root-module output holding sensitive data unless it is marked sensitive.
func TestOutputsTF_sensitiveComputedAttribute(t *testing.T) {
	s := rsschema.Schema{Attributes: map[string]rsschema.Attribute{
		"id":     rsschema.StringAttribute{Computed: true},
		"secret": rsschema.StringAttribute{Computed: true, Sensitive: true},
	}}
	_, outs, _ := collect("kion_example", s)
	got := outputsTF("kion_example", outs)

	secret := got[strings.Index(got, `output "secret"`):]
	if !strings.Contains(secret, "sensitive   = true") {
		t.Errorf("secret output is not marked sensitive:\n%s", got)
	}
	id := got[strings.Index(got, `output "id"`):strings.Index(got, `output "secret"`)]
	if strings.Contains(id, "sensitive") {
		t.Errorf("id output is marked sensitive:\n%s", got)
	}
}
