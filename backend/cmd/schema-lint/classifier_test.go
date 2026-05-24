package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// helper returns a zero-value ClassifierConfig (no interface-to-impl mappings).
func noIfaceConfig() ClassifierConfig { return ClassifierConfig{} }

// makeAllowlist constructs an Allowlist from a slice of mutation names.
func makeAllowlist(names ...string) Allowlist {
	al := make(Allowlist, len(names))
	for _, n := range names {
		al[n] = struct{}{}
	}
	return al
}

// --- Classify tests ----------------------------------------------------------

func TestClassify_SingleBareEmit_NoAllowlist_OneViolation(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
	}
	fieldMap := map[string]string{
		"AdminRoleUC": "adminRoleUsecase",
	}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d: %v", len(drifts), drifts)
	}
	want := []Violation{
		{
			Mutation:       "updateRole",
			ReturnType:     "Role!",
			ResolverMethod: "UpdateRole",
			UsecaseTarget:  "AdminRoleUC.Update",
		},
	}
	if diff := cmp.Diff(want, violations); diff != "" {
		t.Errorf("violations mismatch (-want +got):\n%s", diff)
	}
}

func TestClassify_SingleBareEmit_InAllowlist_ZeroViolations(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist("updateRole")

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations (allowlisted), got %d: %v", len(violations), violations)
	}
}

func TestClassify_UnionReturn_ZeroViolations(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "createCard", ReturnType: "CreateCardResult!", IsBare: false},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "createCard",
				ResolverMethod: "CreateCard",
				UsecaseCalls: []UsecaseCall{
					{Selector: "CardUC", Method: "Create"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*cardUsecase", Method: "Create", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"CardUC": "cardUsecase"}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for union return, got %d: %v", len(violations), violations)
	}
}

func TestClassify_ListReturn_ZeroViolations(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "bulkCreateCards", ReturnType: "[Card!]!", IsBare: false},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "bulkCreateCards",
				ResolverMethod: "BulkCreateCards",
				UsecaseCalls: []UsecaseCall{
					{Selector: "CardUC", Method: "BulkCreate"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*cardUsecase", Method: "BulkCreate", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"CardUC": "cardUsecase"}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for list return, got %d: %v", len(violations), violations)
	}
}

func TestClassify_ScalarReturn_ZeroViolations(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "deleteRole", ReturnType: "Boolean!", IsBare: false},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "deleteRole",
				ResolverMethod: "DeleteRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Delete"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Delete", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for scalar return, got %d: %v", len(violations), violations)
	}
}

func TestClassify_BareReturn_NoEmission_ZeroViolations(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: false},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations when usecase does not emit, got %d: %v", len(violations), violations)
	}
}

func TestClassify_SchemaSideDrift_NoResolverMethod(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{}, // no matching resolver
	}
	usecase := []UsecaseMethod{}
	fieldMap := map[string]string{}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(violations) != 0 {
		t.Errorf("expected 0 violations for drift-only scenario, got %d", len(violations))
	}
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d: %v", len(drifts), drifts)
	}
	if drifts[0].Mutation != "updateRole" {
		t.Errorf("drift.Mutation = %q, want %q", drifts[0].Mutation, "updateRole")
	}
	if drifts[0].ResolverMethod != "" {
		t.Errorf("drift.ResolverMethod = %q, want empty for schema-side drift", drifts[0].ResolverMethod)
	}
}

func TestClassify_ResolverSideDrift_NoMutationField(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{} // no mutation declared
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(violations) != 0 {
		t.Errorf("expected 0 violations for drift-only scenario, got %d", len(violations))
	}
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d: %v", len(drifts), drifts)
	}
	if drifts[0].ResolverMethod != "UpdateRole" {
		t.Errorf("drift.ResolverMethod = %q, want %q", drifts[0].ResolverMethod, "UpdateRole")
	}
	if drifts[0].Mutation != "" {
		t.Errorf("drift.Mutation = %q, want empty for resolver-side drift", drifts[0].Mutation)
	}
}

func TestClassify_MultiCallResolver_EmissionORAggregation(t *testing.T) {
	t.Parallel()

	// First call does NOT emit; second call DOES emit.
	schema := []MutationReturn{
		{Field: "handleSwipe", ReturnType: "Card!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "handleSwipe",
				ResolverMethod: "HandleSwipe",
				UsecaseCalls: []UsecaseCall{
					{Selector: "CardgroupUC", Method: "Get"},
					{Selector: "CardUC", Method: "Create"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*cardgroupUsecase", Method: "Get", EmitsTypedError: false},
		{ReceiverType: "*cardUsecase", Method: "Create", EmitsTypedError: true},
	}
	fieldMap := map[string]string{
		"CardgroupUC": "cardgroupUsecase",
		"CardUC":      "cardUsecase",
	}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation from OR-aggregation, got %d: %v", len(violations), violations)
	}
	if violations[0].Mutation != "handleSwipe" {
		t.Errorf("violation.Mutation = %q, want %q", violations[0].Mutation, "handleSwipe")
	}
}

// TestClassify_MultiCallResolver_EmissionORAggregation_ReverseDirection asserts
// the symmetric case: first call DOES emit, second call does NOT. The result
// must still be a single violation, proving OR semantics are not order-dependent.
func TestClassify_MultiCallResolver_EmissionORAggregation_ReverseDirection(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "handleSwipe", ReturnType: "Card!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "handleSwipe",
				ResolverMethod: "HandleSwipe",
				UsecaseCalls: []UsecaseCall{
					{Selector: "CardUC", Method: "Create"},   // emits
					{Selector: "CardgroupUC", Method: "Get"}, // does not emit
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*cardUsecase", Method: "Create", EmitsTypedError: true},
		{ReceiverType: "*cardgroupUsecase", Method: "Get", EmitsTypedError: false},
	}
	fieldMap := map[string]string{
		"CardUC":      "cardUsecase",
		"CardgroupUC": "cardgroupUsecase",
	}
	allowlist := makeAllowlist()

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation from OR-aggregation, got %d: %v", len(violations), violations)
	}
	if violations[0].Mutation != "handleSwipe" {
		t.Errorf("violation.Mutation = %q, want %q", violations[0].Mutation, "handleSwipe")
	}
}

func TestClassify_InterfaceToImplResolution(t *testing.T) {
	t.Parallel()

	// AdminRoleUC is declared as interface type AdminRoleUsecase in the resolver
	// struct. The impl is adminRoleUsecase.
	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	// LoadResolverFieldMap returns the interface name; impl differs.
	fieldMap := map[string]string{
		"AdminRoleUC": "AdminRoleUsecase", // interface name from resolver.go
	}
	usecase := []UsecaseMethod{
		// The method is on the *impl*, not the interface.
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
	}
	allowlist := makeAllowlist()
	cfg := ClassifierConfig{
		InterfaceToImpl: map[string]string{
			"AdminRoleUsecase": "adminRoleUsecase",
		},
	}

	violations, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, cfg)

	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation via interface->impl resolution, got %d: %v", len(violations), violations)
	}
	if violations[0].Mutation != "updateRole" {
		t.Errorf("violation.Mutation = %q, want %q", violations[0].Mutation, "updateRole")
	}
}

// TestClassify_UnresolvableSelector_EmitsDrift asserts that a resolver method
// whose usecase selector cannot be resolved via the field map (and is also
// not present in InterfaceToImpl) produces a classifier drift. This prevents
// the silent failure mode where a new interface-typed Resolver field is
// added but the hardcoded interfaceToImpl map is not extended.
func TestClassify_UnresolvableSelector_EmitsDrift(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "frobnicate", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "frobnicate",
				ResolverMethod: "Frobnicate",
				UsecaseCalls: []UsecaseCall{
					{Selector: "MysteryUC", Method: "DoIt"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{}
	// Field map deliberately omits MysteryUC.
	fieldMap := map[string]string{}
	allowlist := makeAllowlist()

	_, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(drifts) == 0 {
		t.Fatalf("expected at least one drift for unresolvable selector, got 0")
	}
	found := false
	for _, d := range drifts {
		if d.ResolverMethod == "Frobnicate" && strings.Contains(d.Description, "no corresponding usecase type found") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a drift mentioning the unresolved selector; got drifts: %v", drifts)
	}
}

// TestClassify_ResolvableSelectorButMissingMethod_NoDrift asserts that when
// the selector resolves but the method does not match any UsecaseMethod entry
// (e.g. the method does not exist on that receiver), the classifier does NOT
// emit a drift for that case — that is a separate "method not in usecase scope"
// concern, not the silent-disable failure the unresolvable-selector drift
// guards against.
func TestClassify_ResolvableSelectorButMissingMethod_NoDrift(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "MethodThatDoesNotExist"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		// Different method name; the call's method is not represented.
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist()

	_, drifts := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	for _, d := range drifts {
		if d.ResolverMethod == "UpdateRole" && strings.Contains(d.Description, "no corresponding usecase type found") {
			t.Errorf("did not expect an unresolvable-selector drift for a method-not-found case; got: %v", d)
		}
	}
}

func TestClassify_SortedOutput_Deterministic(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
		{Field: "createRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
			{
				MutationField:  "createRole",
				ResolverMethod: "CreateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Create"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
		{ReceiverType: "*adminRoleUsecase", Method: "Create", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist()

	violations, _ := Classify(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}
	// Must be sorted: createRole < updateRole.
	if violations[0].Mutation != "createRole" || violations[1].Mutation != "updateRole" {
		t.Errorf("violations not sorted: got [%s, %s]", violations[0].Mutation, violations[1].Mutation)
	}
}

// --- LoadAllowlist tests -----------------------------------------------------

func TestLoadAllowlist_BlankLinesAndComments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.txt")
	content := `# This is a comment line
# Another comment

updateRole    # inline comment
createRole

# section header
deleteRole
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	al, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}

	want := makeAllowlist("updateRole", "createRole", "deleteRole")
	if diff := cmp.Diff(want, al); diff != "" {
		t.Errorf("allowlist mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadAllowlist_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.txt")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	al, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}
	if len(al) != 0 {
		t.Errorf("expected empty allowlist, got %d entries", len(al))
	}
}

func TestLoadAllowlist_OnlyComments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.txt")
	content := "# comment one\n# comment two\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	al, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}
	if len(al) != 0 {
		t.Errorf("expected empty allowlist for comment-only file, got %d entries", len(al))
	}
}

func TestLoadAllowlist_InlineComment(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.txt")
	content := "updateRole   # promoted later\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	al, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}
	if _, ok := al["updateRole"]; !ok {
		t.Errorf("expected 'updateRole' in allowlist after stripping inline comment")
	}
	if len(al) != 1 {
		t.Errorf("expected 1 entry, got %d", len(al))
	}
}

func TestLoadAllowlist_NonExistentFile(t *testing.T) {
	t.Parallel()

	_, err := LoadAllowlist(filepath.Join(t.TempDir(), "does-not-exist.txt"))
	if err == nil {
		t.Fatal("LoadAllowlist: expected error for missing file, got nil")
	}
}

// --- LoadResolverFieldMap tests ----------------------------------------------

func TestLoadResolverFieldMap(t *testing.T) {
	t.Parallel()

	// Write a minimal resolver.go fixture to a temp file.
	dir := t.TempDir()
	resolverSrc := `package resolver

import "backend/internal/usecase"

type Resolver struct {
	CardUC      *usecase.CardUsecase
	AdminRoleUC usecase.AdminRoleUsecase
}
`
	path := filepath.Join(dir, "resolver.go")
	if err := os.WriteFile(path, []byte(resolverSrc), 0o644); err != nil {
		t.Fatalf("write resolver fixture: %v", err)
	}

	got, err := LoadResolverFieldMap(path)
	if err != nil {
		t.Fatalf("LoadResolverFieldMap: %v", err)
	}

	want := map[string]string{
		"CardUC":      "CardUsecase",      // *usecase.CardUsecase -> "CardUsecase"
		"AdminRoleUC": "AdminRoleUsecase", // usecase.AdminRoleUsecase -> "AdminRoleUsecase"
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("field map mismatch (-want +got):\n%s", diff)
	}
}

// --- AllowlistRotEntries tests -----------------------------------------------

func TestAllowlistRotEntries_PromotedMutation_IsRotten(t *testing.T) {
	t.Parallel()

	// The mutation now returns a union (IsBare=false) but is still in allowlist.
	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "UpdateRoleResult!", IsBare: false},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist("updateRole")

	rotten := AllowlistRotEntries(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(rotten) != 1 || rotten[0] != "updateRole" {
		t.Errorf("expected [\"updateRole\"] as rotten, got %v", rotten)
	}
}

func TestAllowlistRotEntries_StillViolating_NotRotten(t *testing.T) {
	t.Parallel()

	// The mutation is still bare and still emits — it should NOT appear as rotten.
	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "updateRole",
				ResolverMethod: "UpdateRole",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist("updateRole")

	rotten := AllowlistRotEntries(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(rotten) != 0 {
		t.Errorf("expected no rotten entries, got %v", rotten)
	}
}

// Test 2: AllowlistRotEntries branch coverage for missing-mutation branch.
// The allowlist has "oldMutation" but the schema slice does NOT contain it.
// AllowlistRotEntries must report "oldMutation" as a rot entry with no further
// context required — the mutation has been removed from the schema entirely.
func TestAllowlistRotEntries_MutationRemovedFromSchema_IsRotten(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		// "oldMutation" is absent; only an unrelated mutation is present.
		{Field: "keepMe", ReturnType: "Thing!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "keepMe",
				ResolverMethod: "KeepMe",
				UsecaseCalls:   nil,
			},
		},
	}
	usecase := []UsecaseMethod{}
	fieldMap := map[string]string{}
	allowlist := makeAllowlist("oldMutation")

	rotten := AllowlistRotEntries(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(rotten) != 1 {
		t.Fatalf("expected 1 rotten entry, got %d: %v", len(rotten), rotten)
	}
	if rotten[0] != "oldMutation" {
		t.Errorf("expected rotten entry %q, got %q", "oldMutation", rotten[0])
	}
}

// Test 2: AllowlistRotEntries branch coverage for missing-resolver branch.
// The allowlist has "mut"; schema has "mut" as a bare return; but the resolver
// mapping slice contains no method whose camelCase name matches "mut".
// AllowlistRotEntries must report "mut" as a rot entry.
func TestAllowlistRotEntries_ResolverMethodRemoved_IsRotten(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "orphanedMut", ReturnType: "Role!", IsBare: true},
	}
	// Resolver has no mapping for "orphanedMut".
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{},
	}
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: true},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist("orphanedMut")

	rotten := AllowlistRotEntries(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(rotten) != 1 {
		t.Fatalf("expected 1 rotten entry, got %d: %v", len(rotten), rotten)
	}
	if rotten[0] != "orphanedMut" {
		t.Errorf("expected rotten entry %q, got %q", "orphanedMut", rotten[0])
	}
}

// Test 2: AllowlistRotEntries branch coverage for no-longer-emits branch.
// The allowlist has "silentMut"; schema has "silentMut" as a bare return;
// resolver matches; but the usecase method now has EmitsTypedError = false.
// AllowlistRotEntries must report "silentMut" as a rot entry.
func TestAllowlistRotEntries_UsecaseNoLongerEmits_IsRotten(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "silentMut", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:  "silentMut",
				ResolverMethod: "SilentMut",
				UsecaseCalls: []UsecaseCall{
					{Selector: "AdminRoleUC", Method: "Update"},
				},
			},
		},
	}
	// Usecase method now has EmitsTypedError = false.
	usecase := []UsecaseMethod{
		{ReceiverType: "*adminRoleUsecase", Method: "Update", EmitsTypedError: false},
	}
	fieldMap := map[string]string{"AdminRoleUC": "adminRoleUsecase"}
	allowlist := makeAllowlist("silentMut")

	rotten := AllowlistRotEntries(schema, resolver, usecase, fieldMap, allowlist, noIfaceConfig())

	if len(rotten) != 1 {
		t.Fatalf("expected 1 rotten entry, got %d: %v", len(rotten), rotten)
	}
	if rotten[0] != "silentMut" {
		t.Errorf("expected rotten entry %q, got %q", "silentMut", rotten[0])
	}
}
