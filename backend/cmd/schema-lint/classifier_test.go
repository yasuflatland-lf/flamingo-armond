package main

import (
	"os"
	"path/filepath"
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
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "createCard",
				ResolverMethod:  "CreateCard",
				UsecaseSelector: "CardUC",
				UsecaseMethod:   "Create",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "bulkCreateCards",
				ResolverMethod:  "BulkCreateCards",
				UsecaseSelector: "CardUC",
				UsecaseMethod:   "BulkCreate",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "deleteRole",
				ResolverMethod:  "DeleteRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Delete",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
		Mappings:        []ResolverMapping{}, // no matching resolver
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "handleSwipe",
				ResolverMethod:  "HandleSwipe",
				UsecaseSelector: "CardgroupUC",
				UsecaseMethod:   "Get",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{
			"HandleSwipe": {
				{Selector: "CardUC", Method: "Create"},
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
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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

func TestClassify_SortedOutput_Deterministic(t *testing.T) {
	t.Parallel()

	schema := []MutationReturn{
		{Field: "updateRole", ReturnType: "Role!", IsBare: true},
		{Field: "createRole", ReturnType: "Role!", IsBare: true},
	}
	resolver := ResolverWalkResult{
		Mappings: []ResolverMapping{
			{
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
			{
				MutationField:   "createRole",
				ResolverMethod:  "CreateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Create",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
assignRole
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	al, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}

	want := makeAllowlist("updateRole", "createRole", "assignRole")
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
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
				MutationField:   "updateRole",
				ResolverMethod:  "UpdateRole",
				UsecaseSelector: "AdminRoleUC",
				UsecaseMethod:   "Update",
			},
		},
		AdditionalCalls: map[string][]struct{ Selector, Method string }{},
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
