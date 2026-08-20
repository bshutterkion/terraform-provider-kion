// Package migratehelper holds the runtime converters the generated state
// upgraders (<name>_upgrade_gen.go) call. The upgraders transform the old-schema
// (SDKv2) state as a raw JSON map into a new-schema JSON map, then decode it with
// tftypes.ValueFromJSON — so nested objects, sets, maps, and scalars all migrate
// uniformly. These helpers operate at the JSON level. It is hand-written
// infrastructure (like internal/flex), not a resource body.
package migratehelper

import "encoding/json"

// Null is the JSON null literal, for new attributes with no old source (the
// provider's next Read repopulates them).
var Null = json.RawMessage("null")

// OrNull returns raw, or JSON null when the old attribute was absent.
func OrNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return Null
	}
	return raw
}

// ProjectIDs maps an old set/list of single-field objects — [{<field>: n}, …] —
// to a JSON array of those field values ([n, …]). This is the block-set → id-list
// transform (owner_users:[{id}] → owner_user_ids:[int]). Null/absent → null.
func ProjectIDs(raw json.RawMessage, field string) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return Null
	}
	var elems []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return Null
	}
	ids := make([]json.RawMessage, 0, len(elems))
	for _, e := range elems {
		if v, ok := e[field]; ok {
			ids = append(ids, v)
		}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return Null
	}
	return b
}

// Unwrap turns an old single-element block set/list — [{…}] — into a single
// object {…}, for blocks that became a SingleNestedAttribute (e.g. aws_account's
// aws_organizational_unit). Empty/absent → null.
func Unwrap(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return Null
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil || len(elems) == 0 {
		return Null
	}
	return elems[0]
}

// StringToNum converts an old string id ("42") to a JSON number (42) — used where
// the new schema declares id as a number (e.g. aws_account). Null/absent → null.
func StringToNum(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return Null
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return raw // already a number
	}
	if s == "" {
		return Null
	}
	return json.RawMessage(s)
}
