package crud

import (
	_ "embed"
	"fmt"
	"os"
	"slices"
)

//go:embed sweep.gtpl
var sweepStubTmpl string

//go:embed sweep_list.gtpl
var sweepRealTmpl string

// renderSweep renders sweep.go. With a resolved list endpoint and a delete op it
// emits a real list+delete-by-prefix sweeper (self-contained: its own pagination
// over the resolved envelope, no coupling to the data source). Otherwise, or on
// build failure, it emits a registered stub so the package stays complete.
func renderSweep(rm ResourceModel) ([]byte, error) {
	if rm.List != nil && rm.Delete != nil {
		b, err := renderRealSweep(rm)
		if err == nil {
			return b, nil
		}
		fmt.Fprintf(os.Stderr, "kgen crud: %s — real sweeper failed (%v); using stub\n", rm.Name, err)
	}
	return renderStubSweep(rm)
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
	Pkg, Pascal, ResourceType  string
	SDKAlias                   string
	ElemType                   string
	ListMethod, ListParams     string
	ListRespType               string
	ItemsGo, TotalGo           string
	PageParam, CountParam      string
	ItemsNil                   bool
	DeleteMethod, DeleteParams string
	DeleteIDParam              string
	DeleteIDType               string   // "int64" | "uint64" — delete param id Go type
	IDSDKGo                    string   // payload id field GoName, e.g. "ID"
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

	respByJSON := map[string]Field{}
	for _, f := range rm.Read.RespFields {
		respByJSON[f.JSONName] = f
	}
	idField, ok := respByJSON["id"]
	if !ok {
		return realSweepData{}, fmt.Errorf("%s sweeper: response has no id field", rm.Name)
	}

	d := realSweepData{
		Pkg:           rm.Name,
		Pascal:        rm.Pascal,
		ResourceType:  "kion_" + rm.Name,
		SDKAlias:      "generated",
		ElemType:      lm.ElemType,
		ListMethod:    lm.Method.Name,
		ListParams:    lm.Method.ParamsType,
		ListRespType:  lm.RespType,
		ItemsGo:       lm.ItemsGo,
		TotalGo:       lm.TotalGo,
		PageParam:     lm.PageParam,
		CountParam:    lm.CountParam,
		ItemsNil:      lm.ItemsNil,
		DeleteMethod:  rm.Delete.Method.Name,
		DeleteParams:  rm.Delete.Method.ParamsType,
		DeleteIDParam: delIDParam,
		DeleteIDType:  delIDType,
		IDSDKGo:       idField.GoName,
		Prefix:        "test-acc",
	}

	// String-valued payload fields drive the prefix match.
	for _, mf := range rm.Fields {
		pf, ok := respByJSON[mf.TFSDK]
		if !ok {
			continue
		}
		switch pf.Type {
		case "string":
			d.MatchExprs = append(d.MatchExprs, "item."+pf.GoName)
		case "OptString":
			d.MatchExprs = append(d.MatchExprs, "item."+pf.GoName+`.Or("")`)
		}
	}
	if len(d.MatchExprs) == 0 {
		return realSweepData{}, fmt.Errorf("%s sweeper: no string field to match the test prefix", rm.Name)
	}
	slices.Sort(d.MatchExprs)
	return d, nil
}
