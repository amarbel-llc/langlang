package extract

import (
	"strings"
	"testing"
)

func TestEmitViewLeaf(t *testing.T) {
	rules := map[string]RuleInfo{
		"Ident": {Name: "Ident", Kind: RuleLeaf, NameID: 0},
	}

	code := emitViewTypes(rules, "Ident")

	checks := []string{
		"type IdentView struct",
		"t *tree",
		"id NodeID",
		"func (v IdentView) Text() string",
		"v.t.UnsafeText(v.id)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("missing %q in:\n%s", c, code)
		}
	}
}

func TestEmitViewChoice(t *testing.T) {
	rules := map[string]RuleInfo{
		"Value": {Name: "Value", Kind: RuleChoice, NameID: 0,
			Choices: []string{"Object", "String"}},
		"Object": {Name: "Object", Kind: RuleSequence, NameID: 1},
		"String": {Name: "String", Kind: RuleLeaf, NameID: 2},
	}

	code := emitViewTypes(rules, "Value")

	checks := []string{
		"type ValueView struct",
		"func (v ValueView) Object() (ObjectView, bool)",
		"func (v ValueView) String() (string, bool)",
		"_nameID_Object",
		"_nameID_String",
		"t.IsNamed(child,",
		"UnsafeText",
		// Choice for sequence child should use constructor
		"newObjectView(v.t, child)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("missing %q in:\n%s", c, code)
		}
	}
}

func TestEmitViewSequence(t *testing.T) {
	rules := map[string]RuleInfo{
		"Member": {Name: "Member", Kind: RuleSequence, NameID: 0,
			Children: []RuleChild{
				{RuleName: "Key", Index: 0},
				{IsLiteral: true, Index: 1},
				{RuleName: "Value", Index: 2},
			}},
		"Key":   {Name: "Key", Kind: RuleLeaf, NameID: 1},
		"Value": {Name: "Value", Kind: RuleChoice, NameID: 2},
	}

	code := emitViewTypes(rules, "Member")

	checks := []string{
		"type MemberView struct",
		// Pre-resolved fields
		"_key NodeID",
		"_hasKey bool",
		"_value NodeID",
		"_hasValue bool",
		// Constructor
		"func newMemberView(t *tree, id NodeID) MemberView",
		"t.childRanges",
		"t.children[i]",
		"case _nameID_Key:",
		"case _nameID_Value:",
		// Leaf accessor is O(1)
		"func (v MemberView) Key() string",
		"v._hasKey",
		"v.t.UnsafeText(v._key)",
		// Non-leaf accessor is O(1)
		"func (v MemberView) Value() (ValueView, bool)",
		"v._hasValue",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("missing %q in:\n%s", c, code)
		}
	}
}

func TestEmitViewSequenceRepeated(t *testing.T) {
	rules := map[string]RuleInfo{
		"Array": {Name: "Array", Kind: RuleSequence, NameID: 0,
			Children: []RuleChild{
				{RuleName: "Value", Index: 0},
				{RuleName: "Value", Index: 1},
			}},
		"Value": {Name: "Value", Kind: RuleChoice, NameID: 1},
	}

	code := emitViewTypes(rules, "Array")

	checks := []string{
		"_value []NodeID",
		"func (v ArrayView) ValueCount() int",
		"func (v ArrayView) ValueAt(i int) ValueView",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("missing %q in:\n%s", c, code)
		}
	}
}

func TestEmitViewRepeat(t *testing.T) {
	rules := map[string]RuleInfo{
		"Items": {Name: "Items", Kind: RuleRepeat, NameID: 0, Inner: "Item"},
		"Item":  {Name: "Item", Kind: RuleSequence, NameID: 1},
	}

	code := emitViewTypes(rules, "Items")

	checks := []string{
		"type ItemsView struct",
		"func (v ItemsView) VisitItem(fn func(ItemView) bool)",
		"_nameID_Item",
		// Direct iteration, no Visit
		"t.childRanges",
		"t.children[i]",
		// Sequence child uses constructor
		"newItemView(v.t, cid)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("missing %q in:\n%s", c, code)
		}
	}
}

func TestEmitViewAlias(t *testing.T) {
	rules := map[string]RuleInfo{
		"Expr": {Name: "Expr", Kind: RuleAlias, NameID: 0, Inner: "Term"},
		"Term": {Name: "Term", Kind: RuleSequence, NameID: 1},
	}

	code := emitViewTypes(rules, "Expr")

	checks := []string{
		"type ExprView struct",
		"func (v ExprView) Term() (TermView, bool)",
		// Sequence child uses constructor
		"newTermView(v.t, child)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("missing %q in:\n%s", c, code)
		}
	}
}

func TestEmitViewOptional(t *testing.T) {
	rules := map[string]RuleInfo{
		"MaybeVal": {Name: "MaybeVal", Kind: RuleOptional, NameID: 0, Inner: "Val"},
		"Val":      {Name: "Val", Kind: RuleSequence, NameID: 1},
	}

	code := emitViewTypes(rules, "MaybeVal")

	checks := []string{
		"type MaybeValView struct",
		"func (v MaybeValView) Val() (ValView, bool)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("missing %q in:\n%s", c, code)
		}
	}
}

func TestEmitViewSkipsNegativeNameID(t *testing.T) {
	rules := map[string]RuleInfo{
		"Recovery": {Name: "Recovery", Kind: RuleLeaf, NameID: -1},
	}

	code := emitViewTypes(rules, "")
	if strings.Contains(code, "RecoveryView") {
		t.Error("should skip rules with NameID < 0")
	}
}

func TestEmitViewSkipsLowercaseRules(t *testing.T) {
	rules := map[string]RuleInfo{
		"arrayClose": {Name: "arrayClose", Kind: RuleLeaf, NameID: 5},
		"eof":        {Name: "eof", Kind: RuleLeaf, NameID: 6},
		"Value":      {Name: "Value", Kind: RuleLeaf, NameID: 0},
	}

	code := emitViewTypes(rules, "")
	if strings.Contains(code, "arrayCloseView") {
		t.Error("should skip lowercase rules")
	}
	if strings.Contains(code, "eofView") {
		t.Error("should skip lowercase rules")
	}
	if !strings.Contains(code, "ValueView") {
		t.Error("should include uppercase rules")
	}
}

func TestEmitViewRootFirst(t *testing.T) {
	rules := map[string]RuleInfo{
		"Zebra": {Name: "Zebra", Kind: RuleLeaf, NameID: 0},
		"Alpha": {Name: "Alpha", Kind: RuleLeaf, NameID: 1},
		"Root":  {Name: "Root", Kind: RuleLeaf, NameID: 2},
	}

	code := emitViewTypes(rules, "Root")
	rootIdx := strings.Index(code, "RootView")
	alphaIdx := strings.Index(code, "AlphaView")
	zebraIdx := strings.Index(code, "ZebraView")

	if rootIdx < 0 || alphaIdx < 0 || zebraIdx < 0 {
		t.Fatal("missing view types")
	}
	if rootIdx > alphaIdx {
		t.Error("root should appear before alpha")
	}
	if alphaIdx > zebraIdx {
		t.Error("alpha should appear before zebra (alphabetical)")
	}
}

func TestRenderViewsFile(t *testing.T) {
	rules := map[string]RuleInfo{
		"Root": {Name: "Root", Kind: RuleLeaf, NameID: 0},
		"Item": {Name: "Item", Kind: RuleLeaf, NameID: 1},
	}

	output, err := RenderViewsFile("mypkg", "test.peg", rules, "Root")
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"package mypkg",
		"DO NOT EDIT",
		"_nameID_Root",
		"_nameID_Item",
		"type RootView struct",
		"type ItemView struct",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("output missing %q", c)
		}
	}
}
