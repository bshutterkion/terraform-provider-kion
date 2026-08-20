package user_group

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestUpgradeState_v0 is the golden fixture: a representative old (SDKv2) state
// upgraded to the new schema. Proves the projected id-lists, null added attrs,
// and scalar passthrough are correct — the "just works" guarantee.
func TestUpgradeState_v0(t *testing.T) {
	ctx := context.Background()
	r := &user_groupResource{}
	up, ok := r.UpgradeState(ctx)[0]
	if !ok {
		t.Fatal("no v0 upgrader")
	}

	oldState := `{
		"id": "42",
		"name": "platform-team",
		"description": "the platform team",
		"enabled": true,
		"idms_id": 1,
		"created_at": "2024-01-01",
		"last_updated": "2024-02-02",
		"owner_users": [{"id": 5}, {"id": 6}],
		"owner_groups": [{"id": 7}],
		"users": [{"id": 9}]
	}`

	schema := UserGroupResourceSchema(ctx)
	req := resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(oldState)}}
	resp := &resource.UpgradeStateResponse{State: tfsdk.State{Schema: schema}}
	up.StateUpgrader(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade diagnostics: %v", resp.Diagnostics)
	}

	var m UserGroupModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("state.Get: %v", resp.Diagnostics)
	}

	// Scalars pass through.
	if got := m.Id.ValueString(); got != "42" {
		t.Errorf("id = %q, want 42", got)
	}
	if got := m.Name.ValueString(); got != "platform-team" {
		t.Errorf("name = %q", got)
	}
	if !m.Enabled.ValueBool() {
		t.Error("enabled should be true")
	}
	if got := m.IdmsId.ValueInt64(); got != 1 {
		t.Errorf("idms_id = %d, want 1", got)
	}

	// Block-sets projected to id lists.
	assertIntList(t, ctx, m.OwnerUserIds, []int64{5, 6}, "owner_user_ids")
	assertIntList(t, ctx, m.OwnerUserGroupIds, []int64{7}, "owner_user_group_ids")
	assertIntList(t, ctx, m.UserIds, []int64{9}, "user_ids")

	// Added attrs are null (provider Read repopulates).
	if !m.ViewerUserIds.IsNull() {
		t.Error("viewer_user_ids should be null")
	}
	if !m.ViewerUserGroupIds.IsNull() {
		t.Error("viewer_user_group_ids should be null")
	}
	if !m.AddSelfAsViewer.IsNull() {
		t.Error("add_self_as_viewer should be null")
	}
}

func assertIntList(t *testing.T, ctx context.Context, l types.List, want []int64, name string) {
	t.Helper()
	var got []int64
	l.ElementsAs(ctx, &got, false)
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", name, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %d, want %d", name, i, got[i], want[i])
		}
	}
}
