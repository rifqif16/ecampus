package api_test

import (
	"encoding/json"
	"regexp"
	"sort"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/rifqif16/ecampus/backend/internal/api"
)

const extPermission = "x-permission"

var (
	specialStatuses = map[string]bool{"public": true, "authenticated": true, "system": true, "signed-url": true}
	permissionKey   = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
)

// permissionValues accepts a string or a list of strings ("any of"), and
// tolerates kin-openapi keeping extension values as raw JSON.
func permissionValues(value any) ([]string, bool) {
	switch v := value.(type) {
	case string:
		return []string{v}, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, len(out) > 0
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err != nil {
			return nil, false
		}
		return permissionValues(decoded)
	default:
		return nil, false
	}
}

func validPermission(value string) bool {
	return specialStatuses[value] || permissionKey.MatchString(value)
}

// operationsWithoutValidPermission returns "METHOD path" for every operation
// whose x-permission is missing or malformed.
func operationsWithoutValidPermission(doc *openapi3.T) []string {
	var bad []string
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			raw, present := op.Extensions[extPermission]
			values, ok := permissionValues(raw)
			if !present || !ok || !allValid(values) {
				bad = append(bad, method+" "+path)
			}
		}
	}
	sort.Strings(bad)
	return bad
}

func allValid(values []string) bool {
	for _, v := range values {
		if !validPermission(v) {
			return false
		}
	}
	return true
}

func TestEmbeddedSpec_EveryOperationDeclaresPermission(t *testing.T) {
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("load embedded spec: %v", err)
	}

	if bad := operationsWithoutValidPermission(doc); len(bad) != 0 {
		t.Fatalf("operations without a valid %s: %v", extPermission, bad)
	}
}

func TestEmbeddedSpec_DeclaresProbeEndpoints(t *testing.T) {
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("load embedded spec: %v", err)
	}

	for _, path := range []string{"/healthz", "/readyz"} {
		if doc.Paths.Value(path) == nil {
			t.Errorf("path %s missing from spec", path)
		}
	}
}

func TestPermissionChecker_DetectsViolations(t *testing.T) {
	const spec = `
openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /ok-special:
    get: {x-permission: public, responses: {"200": {description: ok}}}
  /ok-key:
    get: {x-permission: krs.read.own, responses: {"200": {description: ok}}}
  /ok-any-of:
    get: {x-permission: [grade.read.own, grade.read.class], responses: {"200": {description: ok}}}
  /missing:
    get: {responses: {"200": {description: ok}}}
  /malformed:
    post: {x-permission: "Not A Key", responses: {"200": {description: ok}}}
  /wrong-type:
    put: {x-permission: 42, responses: {"200": {description: ok}}}
  /empty-list:
    delete: {x-permission: [], responses: {"200": {description: ok}}}
`
	doc, err := openapi3.NewLoader().LoadFromData([]byte(spec))
	if err != nil {
		t.Fatalf("load synthetic spec: %v", err)
	}

	got := operationsWithoutValidPermission(doc)

	want := []string{"DELETE /empty-list", "GET /missing", "POST /malformed", "PUT /wrong-type"}
	if len(got) != len(want) {
		t.Fatalf("violations = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("violations = %v, want %v", got, want)
		}
	}
}
