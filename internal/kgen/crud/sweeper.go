package crud

import (
	_ "embed"
	"fmt"
	"slices"
)

//go:embed sweep.gtpl
var sweepStubTmpl string

//go:embed sweep_list.gtpl
var sweepRealTmpl string

// renderSweep renders sweep.go. With a resolved list endpoint and a delete op it
// emits a real list+delete-by-prefix sweeper (self-contained: its own traversal
// of the resolved envelope, no coupling to the data source). Otherwise, or on
// build failure, it emits a registered stub so the package stays complete.
//
// The returned note is non-empty when a real sweeper was expected (list + delete
// both present) but could not be built.
func renderSweep(rm ResourceModel) (out []byte, note string, err error) {
	if rm.List != nil && rm.Delete != nil {
		b, berr := renderRealSweep(rm)
		if berr == nil {
			return b, "", nil
		}
		note = berr.Error()
	}
	b, err := renderStubSweep(rm)
	return b, note, err
}

// --- stub sweeper (degradation) ---

type sweepData struct {
	Pkg          string
	Pascal       string
	ResourceType string
	DeleteMethod string // "" when the resource has no delete op
}

func renderStubSweep(rm ResourceModel) ([]byte, error) {
	d := sweepData{
		Pkg:          rm.Name,
		Pascal:       rm.Pascal,
		ResourceType: "kion_" + rm.Name,
	}
	if rm.Delete != nil {
		d.DeleteMethod = rm.Delete.Method.Name
	}
	return execGoTemplate("sweep", sweepStubTmpl, d, "sweep.go")
}

// --- real sweeper ---

type realSweepData struct {
	listAccess
	Pkg, Pascal, ResourceType  string
	SDKAlias                   string
	ElemType                   string
	ListMethod, ListParams     string
	ListRespType               string
	PageParam, CountParam      string
	DeleteMethod, DeleteParams string
	DeleteIDParam              string
	DeleteIDType               string   // "int64" | "uint64" — delete param id Go type
	IDSDKGo                    string   // element id access path, e.g. "ID" or "Cft.Value.ID"
	MatchExprs                 []string // string field accessors, e.g. "item.Key"
	Prefix                     string
}

func renderRealSweep(rm ResourceModel) ([]byte, error) {
	d, err := buildRealSweepData(rm)
	if err != nil {
		return nil, err
	}
	return execGoTemplate("sweep_real", sweepRealTmpl, d, "sweep.go")
}

func buildRealSweepData(rm ResourceModel) (realSweepData, error) {
	lm := rm.List
	delIDParam, delIDType, err := idParamName(rm.Delete.Params)
	if err != nil {
		return realSweepData{}, fmt.Errorf("%s sweeper: %w", rm.Name, err)
	}

	view := buildRecordView(rm)
	idField, ok := view.Fields["id"]
	if !ok {
		return realSweepData{}, fmt.Errorf("%s sweeper: response has no id field", rm.Name)
	}
	if !listIDOK(idField.Type) {
		return realSweepData{}, fmt.Errorf("%s sweeper: id field type %q unsupported", rm.Name, idField.Type)
	}

	d := realSweepData{
		listAccess:    lm.access(),
		Pkg:           rm.Name,
		Pascal:        rm.Pascal,
		ResourceType:  "kion_" + rm.Name,
		SDKAlias:      "generated",
		ElemType:      lm.ElemType,
		ListMethod:    lm.Method.Name,
		ListParams:    lm.Method.ParamsType,
		ListRespType:  lm.RespType,
		PageParam:     lm.PageParam,
		CountParam:    lm.CountParam,
		DeleteMethod:  rm.Delete.Method.Name,
		DeleteParams:  rm.Delete.Method.ParamsType,
		DeleteIDParam: delIDParam,
		DeleteIDType:  delIDType,
		IDSDKGo:       view.Paths["id"],
		Prefix:        "test-acc",
	}

	// String-valued payload fields drive the prefix match.
	for _, mf := range rm.Fields {
		pf, ok := view.Fields[mf.TFSDK]
		if !ok {
			continue
		}
		switch pf.Type {
		case "string":
			d.MatchExprs = append(d.MatchExprs, "item."+view.Paths[mf.TFSDK])
		case "OptString":
			d.MatchExprs = append(d.MatchExprs, "item."+view.Paths[mf.TFSDK]+`.Or("")`)
		}
	}
	if len(d.MatchExprs) == 0 {
		return realSweepData{}, fmt.Errorf("%s sweeper: no string field to match the test prefix", rm.Name)
	}
	slices.Sort(d.MatchExprs)
	return d, nil
}
