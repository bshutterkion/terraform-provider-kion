package crud

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// opRef is one (method, path) op reference from generator_config.yaml.
type opRef struct {
	Method string
	Path   string
}

// resOps is the resource op-set for one resource.
type resOps struct {
	Create, Read, Update, Delete *opRef
}

// dsOps is the data-source op-set for one resource.
type dsOps struct {
	Read *opRef
}

// --- config loading (codegen/generator_config.yaml, the merged committed op-set) ---

type cfgOp struct {
	Path   string `yaml:"path"`
	Method string `yaml:"method"`
}

func (o *cfgOp) toRef() *opRef {
	if o == nil || o.Path == "" || o.Method == "" {
		return nil
	}
	return &opRef{Method: o.Method, Path: o.Path}
}

type cfgEntry struct {
	Create *cfgOp `yaml:"create"`
	Read   *cfgOp `yaml:"read"`
	Update *cfgOp `yaml:"update"`
	Delete *cfgOp `yaml:"delete"`
}

type cfgFile struct {
	Resources   map[string]cfgEntry `yaml:"resources"`
	DataSources map[string]cfgEntry `yaml:"data_sources"`
}

// heuristicLine matches a resource key annotated for review, e.g.
// "  guessed:  # heuristic — verify". Such resources are not safe to generate
// until a human verifies their ops, so the loader flags them for skipping.
var heuristicLine = regexp.MustCompile(`(?m)^\s{2}([A-Za-z0-9_]+):\s*#\s*(heuristic|INCOMPLETE)`)

// loadConfig parses the merged generator config. INCOMPLETE resources are
// comments (never unmarshalled); resources annotated "# heuristic" are returned
// in flagged so the caller can refuse them per the op-mapping precondition.
func loadConfig(path string) (resources map[string]resOps, dataSources map[string]dsOps, flagged map[string]bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cf cfgFile
	if err := yaml.Unmarshal(raw, &cf); err != nil {
		return nil, nil, nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	resources = make(map[string]resOps, len(cf.Resources))
	for name, e := range cf.Resources {
		resources[name] = resOps{
			Create: e.Create.toRef(),
			Read:   e.Read.toRef(),
			Update: e.Update.toRef(),
			Delete: e.Delete.toRef(),
		}
	}
	dataSources = make(map[string]dsOps, len(cf.DataSources))
	for name, e := range cf.DataSources {
		dataSources[name] = dsOps{Read: e.Read.toRef()}
	}
	flagged = map[string]bool{}
	for _, m := range heuristicLine.FindAllStringSubmatch(string(raw), -1) {
		flagged[m[1]] = true
	}
	return resources, dataSources, flagged, nil
}

// loadVersionSupport reads codegen/version_support.yaml and returns the set of
// version-gated resource names. A missing file yields an empty set.
func loadVersionSupport(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("reading version support %s: %w", path, err)
	}
	var vs struct {
		Resources map[string]struct {
			Min string `yaml:"min"`
			Max string `yaml:"max"`
		} `yaml:"resources"`
	}
	if err := yaml.Unmarshal(raw, &vs); err != nil {
		return nil, fmt.Errorf("parsing version support %s: %w", path, err)
	}
	gated := make(map[string]bool, len(vs.Resources))
	for name := range vs.Resources {
		gated[name] = true
	}
	return gated, nil
}

// --- SDK index ---

type sdkIndex struct {
	methods     map[string]ClientMethod // keyed by "METHOD path"
	structs     map[string]Struct       // schemas + parameters merged
	markerImpls map[string][]string
}

// buildIndex parses the SDK's generated Go for one version into an sdkIndex.
func buildIndex(src Source, sdkDir, version string) (sdkIndex, error) {
	if version == "" {
		version = "v3_16"
	}
	gen := filepath.Join(sdkDir, "generated", version)
	cms, err := src.ClientMethods(filepath.Join(gen, "oas_client_gen.go"))
	if err != nil {
		return sdkIndex{}, err
	}
	methods := make(map[string]ClientMethod, len(cms))
	for _, m := range cms {
		methods[m.HTTPMethod+" "+m.Path] = m
	}
	schemas, err := src.Structs(filepath.Join(gen, "oas_schemas_gen.go"))
	if err != nil {
		return sdkIndex{}, err
	}
	params, err := src.Structs(filepath.Join(gen, "oas_parameters_gen.go"))
	if err != nil {
		return sdkIndex{}, err
	}
	structs := maps.Clone(schemas)
	maps.Copy(structs, params)
	markers, err := src.MarkerImpls(filepath.Join(gen, "oas_schemas_gen.go"))
	if err != nil {
		return sdkIndex{}, err
	}
	return sdkIndex{methods: methods, structs: structs, markerImpls: markers}, nil
}

// sharedResponse names the ogen result-union members that are NOT the resource
// payload envelope (shared error/status responses). Resolving a read's success
// envelope is "the one impl that is not in this set".
var sharedResponse = map[string]bool{
	"BadRequestResponse":          true,
	"UnauthorizedResponse":        true,
	"ForbiddenResponse":           true,
	"NotFoundResponse":            true,
	"ConflictResponse":            true,
	"UnprocessableEntityResponse": true,
	"TooManyRequestsResponse":     true,
	"InternalServerErrorResponse": true,
	"CreatedResponse":             true,
	"NoContentResponse":           true,
}

// noSDKOpError signals a mapped op that the SDK does not cover (private endpoint).
// The entity generator refuses these; raw-HTTP support is a follow-up.
type noSDKOpError struct {
	verb, method, path string
}

func (e noSDKOpError) Error() string {
	return fmt.Sprintf("%s op %s %s not found in SDK (private endpoint? raw-HTTP not yet supported)", e.verb, e.method, e.path)
}

// --- list envelope resolution ---

// sliceElem returns the element of a rendered slice type ("[]Label" -> "Label").
func sliceElem(typ string) (string, bool) {
	return strings.CutPrefix(typ, "[]")
}

// wrapperSliceElem resolves a field type that wraps a slice (e.g.
// OptNilLabelArray -> Value []Label -> Label) and reports whether the wrapper
// carries a .Null field. Returns ok=false when the type is not a slice wrapper.
func wrapperSliceElem(typeName string, idx sdkIndex) (elem string, nilAware, ok bool) {
	if e, isSlice := sliceElem(typeName); isSlice {
		return e, false, true
	}
	st, found := idx.structs[typeName]
	if !found {
		return "", false, false
	}
	for _, f := range st.Fields {
		if f.GoName != "Value" {
			continue
		}
		e, isSlice := sliceElem(f.Type)
		if !isSlice {
			return "", false, false
		}
		for _, g := range st.Fields {
			if g.GoName == "Null" {
				nilAware = true
			}
		}
		return e, nilAware, true
	}
	return "", false, false
}

// resolveList resolves a paginated-list op (the data-source read) into a
// listModel: the success response, its Data envelope, the items slice field
// whose element equals the read payload, the Total field, and the Page/Count
// params. Any gap returns an error so the caller degrades to id-only output.
func resolveList(ref *opRef, idx sdkIndex, payloadType string) (*listModel, error) {
	if ref == nil {
		return nil, fmt.Errorf("no data-source list op configured")
	}
	m, ok := idx.methods[ref.Method+" "+ref.Path]
	if !ok {
		return nil, noSDKOpError{verb: "list", method: ref.Method, path: ref.Path}
	}

	marker := lowerFirst(m.ResultType)
	var payload []string
	for _, t := range idx.markerImpls[marker] {
		if !sharedResponse[t] {
			payload = append(payload, t)
		}
	}
	if len(payload) != 1 {
		return nil, fmt.Errorf("list envelope: expected one payload impl for %s, got %v", marker, payload)
	}
	respType := payload[0]

	inner := ""
	for _, f := range idx.structs[respType].Fields {
		if f.GoName == "Data" || f.JSONName == "data" {
			inner = strings.TrimPrefix(f.Type, "Opt")
		}
	}
	if inner == "" {
		return nil, fmt.Errorf("list envelope %s has no Data field", respType)
	}
	innerStruct, ok := idx.structs[inner]
	if !ok {
		return nil, fmt.Errorf("list envelope inner %s not found", inner)
	}

	lm := &listModel{Method: m, RespType: respType, DataInner: inner, ElemType: payloadType}
	if m.ParamsType != "" {
		if p, ok := idx.structs[m.ParamsType]; ok {
			lm.Params = &p
		}
	}
	for _, f := range innerStruct.Fields {
		if f.GoName == "Total" {
			lm.TotalGo = "Total"
		}
		if elem, nilAware, ok := wrapperSliceElem(f.Type, idx); ok && elem == payloadType {
			lm.ItemsGo = f.GoName
			lm.ItemsNil = nilAware
		}
	}
	if lm.ItemsGo == "" {
		return nil, fmt.Errorf("list envelope %s has no []%s items field", inner, payloadType)
	}
	if lm.Params != nil {
		for _, f := range lm.Params.Fields {
			switch f.GoName {
			case "Page":
				lm.PageParam = "Page"
			case "Count":
				lm.CountParam = "Count"
			}
		}
	}
	if lm.PageParam == "" || lm.CountParam == "" {
		return nil, fmt.Errorf("list op %s params lack Page/Count", m.Name)
	}
	return lm, nil
}

// --- resolution ---

// resolveOp resolves one verb's op-ref against the SDK index into an OpModel.
// Returns (nil, nil) when ref is nil (verb absent).
func resolveOp(verb string, ref *opRef, idx sdkIndex) (*OpModel, error) {
	if ref == nil {
		return nil, nil
	}
	m, ok := idx.methods[ref.Method+" "+ref.Path]
	if !ok {
		return nil, noSDKOpError{verb: verb, method: ref.Method, path: ref.Path}
	}
	op := &OpModel{Verb: verb, Method: m}
	if m.BodyType != "" {
		if b, ok := idx.structs[strings.TrimPrefix(m.BodyType, "Opt")]; ok {
			op.Body = &b
		}
	}
	if m.ParamsType != "" {
		if p, ok := idx.structs[m.ParamsType]; ok {
			op.Params = &p
		}
	}
	if verb == "read" {
		respType, payload, dataPtr, respFields, err := resolveEnvelope(m.ResultType, idx)
		if err != nil {
			return nil, err
		}
		op.RespType = respType
		op.RespPayload = payload
		op.RespDataPtr = dataPtr
		op.RespFields = respFields
	}
	if verb == "create" {
		// A create that returns the record envelope (e.g. *ScopeResponse) rather
		// than a bare *CreatedResponse lets the id be read from the response.
		// Best-effort: on failure the entity template falls back to errs.CreatedID.
		if respType, payload, dataPtr, respFields, err := resolveEnvelope(m.ResultType, idx); err == nil {
			op.RespType = respType
			op.RespPayload = payload
			op.RespDataPtr = dataPtr
			op.RespFields = respFields
		}
	}
	return op, nil
}

// resolveEnvelope finds the read success response type (the non-error impl of
// the result marker), the domain payload type inside its Data envelope, the
// payload's fields, and whether the Data field is a pointer (*Payload) rather
// than an Opt-wrapper.
func resolveEnvelope(resultType string, idx sdkIndex) (respType, payload string, dataPtr bool, fields []Field, err error) {
	marker := lowerFirst(resultType)
	var candidates []string
	for _, t := range idx.markerImpls[marker] {
		if !sharedResponse[t] {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) != 1 {
		return "", "", false, nil, fmt.Errorf("read envelope: expected exactly one payload impl for %s, got %v", marker, candidates)
	}
	respType = candidates[0]
	rs := idx.structs[respType]
	for _, f := range rs.Fields {
		if f.GoName == "Data" || f.JSONName == "data" {
			inner := strings.TrimPrefix(f.Type, "Opt")
			if p, ok := idx.structs[inner]; ok {
				return respType, inner, f.Ptr, p.Fields, nil
			}
		}
	}
	// Flat response with no Data envelope: the payload is the response itself.
	return respType, respType, false, rs.Fields, nil
}

// resolveResource assembles a ResourceModel for the entity archetype. ds is the
// data-source op-set; when its list op resolves, rm.List is populated for the
// dual-mode data source and real sweeper (nil otherwise — a logged degradation).
func resolveResource(name string, ops resOps, ds dsOps, idx sdkIndex, model []ModelField) (ResourceModel, error) {
	pascal := pascalCase(name)
	rm := ResourceModel{Name: name, Pascal: pascal, Model: pascal + "Model"}

	create, err := resolveOp("create", ops.Create, idx)
	if err != nil {
		return rm, err
	}
	read, err := resolveOp("read", ops.Read, idx)
	if err != nil {
		return rm, err
	}
	if create == nil || read == nil {
		return rm, fmt.Errorf("%s: entity archetype requires both create and read ops", name)
	}
	rm.Create, rm.Read = *create, *read

	if rm.Update, err = resolveOp("update", ops.Update, idx); err != nil {
		return rm, err
	}
	if rm.Delete, err = resolveOp("delete", ops.Delete, idx); err != nil {
		return rm, err
	}

	for _, mf := range model {
		if mf.TFSDK == "id" {
			rm.IDField = mf
			continue
		}
		rm.Fields = append(rm.Fields, mf)
	}
	if rm.IDField.TFSDK == "" {
		return rm, fmt.Errorf("%s: generated model %s has no tfsdk:\"id\" field", name, rm.Model)
	}

	// Detect a record wrapper in the read payload (the flat model's fields nested
	// under a sub-object, e.g. CFTWithOwnersAndTags.cft).
	detectRecordWrapper(&rm.Read, rm.IDField, rm.Fields, idx)

	// The list element must be the domain payload the single read returns, so the
	// dual-mode DS and sweeper reuse the same field mapping.
	if lm, err := resolveList(ds.Read, idx, rm.Read.RespPayload); err != nil {
		fmt.Fprintf(os.Stderr, "kgen crud: %s — id-only data source + stub sweeper: %v\n", name, err)
	} else {
		rm.List = lm
	}
	return rm, nil
}

// detectRecordWrapper finds a read-payload field that holds the flat model's
// fields as a nested sub-object (a "record wrapper"), setting read.RespWrapper*.
// The distinguishing test: the field's own json-name is NOT a model attribute
// (so it isn't a nested Value attribute like azure_policy's azure_policy), but
// several of the sub-object's fields ARE model attributes. Only the first such
// wrapper is used.
func detectRecordWrapper(read *OpModel, idField ModelField, fields []ModelField, idx sdkIndex) {
	byTF := map[string]ModelField{idField.TFSDK: idField}
	for _, f := range fields {
		byTF[f.TFSDK] = f
	}
	// How many model attributes the TOP-LEVEL payload already provides. A wrapper
	// is only the record holder when it covers MORE than the top level does —
	// otherwise a mere sub-record (e.g. scope's active_criteria_record, which has
	// id+criteria) would be mistaken for the wrapper and duplicate the id.
	topMatched := 0
	for _, f := range read.RespFields {
		if _, ok := byTF[f.JSONName]; ok {
			topMatched++
		}
	}
	bestGo, bestOpt, bestMatched := "", false, topMatched
	var bestFields []Field
	for _, f := range read.RespFields {
		if _, isModelAttr := byTF[f.JSONName]; isModelAttr {
			continue // a nested Value attribute, not a wrapper
		}
		base := strings.TrimPrefix(f.Type, "Opt")
		st, ok := idx.structs[base]
		if !ok || len(st.Fields) == 0 {
			continue
		}
		matched := 0
		for _, sf := range st.Fields {
			if _, ok := byTF[sf.JSONName]; ok {
				matched++
			}
		}
		if matched >= 2 && matched > bestMatched {
			bestGo, bestOpt, bestMatched, bestFields = f.GoName, strings.HasPrefix(f.Type, "Opt"), matched, st.Fields
		}
	}
	read.RespWrapperGo = bestGo
	read.RespWrapperOpt = bestOpt
	read.RespWrapperFields = bestFields
}

// --- small helpers ---

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// pascalCase title-cases each snake segment WITHOUT acronym expansion, matching
// tfplugingen's generated type names (ou_note -> OuNote, gcp_account ->
// GcpAccount), so <Pascal>Model / <Pascal>ResourceSchema line up.
func pascalCase(snake string) string {
	parts := strings.Split(snake, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// lowerCamelCase turns a snake_case name into lowerCamelCase
// ("aws_account" -> "awsAccount", "account" -> "account").
func lowerCamelCase(snake string) string {
	p := pascalCase(snake)
	if p == "" {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}

// spaceBeforeCaps inserts a space before each interior uppercase letter
// ("AwsAccount" -> "Aws Account", "Account" -> "Account").
func spaceBeforeCaps(pascal string) string {
	var b strings.Builder
	for i, r := range pascal {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
