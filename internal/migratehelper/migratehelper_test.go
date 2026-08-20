package migratehelper

import (
	"encoding/json"
	"testing"
)

func eq(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestOrNull(t *testing.T) {
	eq(t, OrNull(nil), "null")
	eq(t, OrNull(json.RawMessage("")), "null")
	eq(t, OrNull(json.RawMessage(`"x"`)), `"x"`)
	eq(t, OrNull(json.RawMessage(`42`)), `42`)
	eq(t, OrNull(json.RawMessage(`false`)), `false`)
	// A JSON null passes through as null (already the right shape).
	eq(t, OrNull(json.RawMessage(`null`)), `null`)
}

func TestProjectIDs(t *testing.T) {
	// The core block-set → id-list transform.
	eq(t, ProjectIDs(json.RawMessage(`[{"id":5},{"id":6}]`), "id"), `[5,6]`)
	// Single element.
	eq(t, ProjectIDs(json.RawMessage(`[{"id":9}]`), "id"), `[9]`)
	// Empty list projects to an empty list, NOT null — an explicitly-empty
	// membership set must survive as [] so it round-trips.
	eq(t, ProjectIDs(json.RawMessage(`[]`), "id"), `[]`)
	// Absent / null source → null (added attr the provider will repopulate).
	eq(t, ProjectIDs(nil, "id"), `null`)
	eq(t, ProjectIDs(json.RawMessage(`null`), "id"), `null`)
	// A different field name.
	eq(t, ProjectIDs(json.RawMessage(`[{"account_id":1},{"account_id":2}]`), "account_id"), `[1,2]`)
	// Objects missing the field are skipped, not errored.
	eq(t, ProjectIDs(json.RawMessage(`[{"id":1},{"other":2}]`), "id"), `[1]`)
	// Malformed JSON degrades to null rather than panicking.
	eq(t, ProjectIDs(json.RawMessage(`{not json`), "id"), `null`)
}

func TestProjectIDs_preservesOrder(t *testing.T) {
	got := ProjectIDs(json.RawMessage(`[{"id":3},{"id":1},{"id":2}]`), "id")
	eq(t, got, `[3,1,2]`)
}

func TestUnwrap(t *testing.T) {
	// [{…}] → {…} for block → single-nested-attribute.
	eq(t, Unwrap(json.RawMessage(`[{"a":1}]`)), `{"a":1}`)
	// Empty list → null.
	eq(t, Unwrap(json.RawMessage(`[]`)), `null`)
	// Absent / null → null.
	eq(t, Unwrap(nil), `null`)
	eq(t, Unwrap(json.RawMessage(`null`)), `null`)
	// More than one element: takes the first (old single-block sets hold ≤1).
	eq(t, Unwrap(json.RawMessage(`[{"a":1},{"a":2}]`)), `{"a":1}`)
	// Malformed → null.
	eq(t, Unwrap(json.RawMessage(`nope`)), `null`)
}

func TestStringToNum(t *testing.T) {
	// The aws_account id string→number case.
	eq(t, StringToNum(json.RawMessage(`"42"`)), `42`)
	// Already a number passes through.
	eq(t, StringToNum(json.RawMessage(`42`)), `42`)
	// Empty string → null (not 0 — the value was effectively unset).
	eq(t, StringToNum(json.RawMessage(`""`)), `null`)
	// Absent / null → null.
	eq(t, StringToNum(nil), `null`)
	eq(t, StringToNum(json.RawMessage(`null`)), `null`)
	// Large numeric id keeps full precision (no float rounding).
	eq(t, StringToNum(json.RawMessage(`"900719925474099"`)), `900719925474099`)
}

// TestStringToNum_roundTripsThroughJSON guards against precision loss: the
// output must be valid JSON that unmarshals back to the same integer.
func TestStringToNum_roundTripsThroughJSON(t *testing.T) {
	out := StringToNum(json.RawMessage(`"12345678901234"`))
	var n json.Number
	if err := json.Unmarshal(out, &n); err != nil {
		t.Fatalf("output not valid JSON number: %v", err)
	}
	if n.String() != "12345678901234" {
		t.Errorf("round-trip = %s, want 12345678901234", n.String())
	}
}
