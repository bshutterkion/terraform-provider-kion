package versions

import (
	"fmt"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// The SDK ships one package per Kion minor, so derivation can only say
// "3.NN.0". A resource that first appeared in a patch release needs the patch
// stated: resource_floors in codegen/attr_version_overrides.yaml raises the
// derived minimum for the resource and its data source. The same code serves
// cmd/kversions, so both outputs agree.

var floorRE = regexp.MustCompile(`^3\.\d+\.\d+$`)

// ParseResourceFloors reads the resource_floors section from the contents of
// attr_version_overrides.yaml. Empty content yields no floors.
func ParseResourceFloors(b []byte) (map[string]string, error) {
	var f attrOverridesFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parsing resource_floors: %w", err)
	}
	for _, name := range sortedResourceNames(f.ResourceFloors) {
		if v := f.ResourceFloors[name]; !floorRE.MatchString(v) {
			return nil, fmt.Errorf("resource_floors: %s: %q is not a 3.N.P version", name, v)
		}
	}
	return f.ResourceFloors, nil
}

// RaiseMin applies floor to a derived minimum. A floor can only tighten: one
// below the derived minimum is an error, as is one above a bounded maximum.
// An empty floor returns derivedMin unchanged.
func RaiseMin(name, derivedMin, derivedMax, floor string) (string, error) {
	if floor == "" {
		return derivedMin, nil
	}
	if versionLess(floor, derivedMin) {
		return "", fmt.Errorf("resource_floors: %s: floor %s is below the derived minimum %s; a floor can only tighten", name, floor, derivedMin)
	}
	if derivedMax != "" && !versionLess(floor, derivedMax) {
		return "", fmt.Errorf("resource_floors: %s: floor %s is not below the derived maximum %s", name, floor, derivedMax)
	}
	return floor, nil
}

// CheckFloorTargets fails when a floor names something that is neither a
// resource nor a data source: a floor nothing reads is worse than none.
func CheckFloorTargets(floors map[string]string, resources, dataSources []string) error {
	known := make(map[string]bool, len(resources)+len(dataSources))
	for _, n := range resources {
		known[n] = true
	}
	for _, n := range dataSources {
		known[n] = true
	}
	for _, name := range sortedResourceNames(floors) {
		if !known[name] {
			return fmt.Errorf("resource_floors: %s (%s) is neither a resource nor a data source in the generator config", name, floors[name])
		}
	}
	return nil
}

// SortedKeys returns the map's keys in order.
func SortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
