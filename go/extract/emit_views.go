package extract

import (
	"fmt"
	"strings"
)

// emitViewTypes generates view types for all named rules in the grammar.
// Views wrap *tree + NodeID and provide typed, read-only access to the parse
// tree. Sequence views pre-resolve child NodeIDs at construction time so
// accessors are O(1) field reads.
func emitViewTypes(rules map[string]RuleInfo, rootRule string) string {
	var buf strings.Builder

	ordered := orderRules(rules, rootRule)

	for _, name := range ordered {
		ri := rules[name]
		emitViewType(&buf, ri, rules)
		buf.WriteString("\n")
	}

	return buf.String()
}

func orderRules(rules map[string]RuleInfo, rootRule string) []string {
	var ordered []string
	seen := map[string]bool{}

	if rootRule != "" {
		if _, ok := rules[rootRule]; ok {
			ordered = append(ordered, rootRule)
			seen[rootRule] = true
		}
	}

	var rest []string
	for name := range rules {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	for i := 0; i < len(rest); i++ {
		for j := i + 1; j < len(rest); j++ {
			if rest[j] < rest[i] {
				rest[i], rest[j] = rest[j], rest[i]
			}
		}
	}
	ordered = append(ordered, rest...)
	return ordered
}

func emitViewType(buf *strings.Builder, ri RuleInfo, rules map[string]RuleInfo) {
	viewName := ri.Name + "View"

	if ri.NameID < 0 {
		return
	}

	switch ri.Kind {
	case RuleSequence:
		emitSequenceView(buf, ri, rules)
	case RuleChoice:
		emitChoiceView(buf, ri, rules)
	case RuleRepeat:
		emitRepeatView(buf, ri, rules)
	case RuleAlias:
		emitAliasView(buf, ri, rules)
	case RuleOptional:
		emitOptionalView(buf, ri, rules)
	case RuleLeaf:
		// Leaf: thin wrapper, just Text()
		fmt.Fprintf(buf, "// %s is a read-only view over a %s node.\n", viewName, ri.Name)
		fmt.Fprintf(buf, "type %s struct {\n", viewName)
		fmt.Fprintf(buf, "\tt *tree\n")
		fmt.Fprintf(buf, "\tid NodeID\n")
		fmt.Fprintf(buf, "}\n\n")
		emitTextMethod(buf, viewName)
	}
}

func emitTextMethod(buf *strings.Builder, viewName string) {
	fmt.Fprintf(buf, "// Text returns the full matched text of this node.\n")
	fmt.Fprintf(buf, "func (v %s) Text() string {\n", viewName)
	fmt.Fprintf(buf, "\treturn v.t.UnsafeText(v.id)\n")
	fmt.Fprintf(buf, "}\n\n")
}

// namedChild describes a non-literal, non-Spacing child in a sequence that
// the view should expose as a field.
type namedChild struct {
	ruleName string
	rule     RuleInfo
	repeated bool
}

// sequenceNamedChildren returns the deduplicated list of named children for
// a sequence rule, excluding literals and Spacing.
func sequenceNamedChildren(ri RuleInfo, rules map[string]RuleInfo) []namedChild {
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, c := range ri.Children {
		if !c.IsLiteral && c.RuleName != "" {
			counts[c.RuleName]++
		}
	}

	var out []namedChild
	for _, c := range ri.Children {
		if c.IsLiteral || c.RuleName == "" || c.RuleName == "Spacing" {
			continue
		}
		if seen[c.RuleName] {
			continue
		}
		seen[c.RuleName] = true

		childRule, ok := rules[c.RuleName]
		if !ok {
			continue
		}
		out = append(out, namedChild{
			ruleName: c.RuleName,
			rule:     childRule,
			repeated: counts[c.RuleName] > 1,
		})
	}
	return out
}

// emitSequenceView generates a struct with pre-resolved NodeID fields for
// each named child, a constructor that walks the sequence once, and O(1)
// accessor methods.
func emitSequenceView(buf *strings.Builder, ri RuleInfo, rules map[string]RuleInfo) {
	viewName := ri.Name + "View"
	children := sequenceNamedChildren(ri, rules)

	// --- struct definition ---
	fmt.Fprintf(buf, "// %s is a read-only view over a %s node.\n", viewName, ri.Name)
	fmt.Fprintf(buf, "// Child references are resolved once at construction time.\n")
	fmt.Fprintf(buf, "type %s struct {\n", viewName)
	fmt.Fprintf(buf, "\tt *tree\n")
	fmt.Fprintf(buf, "\tid NodeID\n")
	for _, nc := range children {
		if nc.repeated {
			fmt.Fprintf(buf, "\t_%s []NodeID\n", fieldName(nc.ruleName))
		} else {
			fmt.Fprintf(buf, "\t_%s NodeID\n", fieldName(nc.ruleName))
			fmt.Fprintf(buf, "\t_has%s bool\n", nc.ruleName)
		}
	}
	fmt.Fprintf(buf, "}\n\n")

	// --- Text ---
	emitTextMethod(buf, viewName)

	// --- constructor ---
	fmt.Fprintf(buf, "func new%s(t *tree, id NodeID) %s {\n", viewName, viewName)
	fmt.Fprintf(buf, "\tv := %s{t: t, id: id}\n", viewName)
	fmt.Fprintf(buf, "\tchild, ok := t.Child(id)\n")
	fmt.Fprintf(buf, "\tif !ok {\n")
	fmt.Fprintf(buf, "\t\treturn v\n")
	fmt.Fprintf(buf, "\t}\n")
	// The child may be a Sequence (multiple children) or a single Node
	// (parser optimizes single-child sequences away).
	fmt.Fprintf(buf, "\tif t.Type(child) == NodeType_Sequence {\n")
	fmt.Fprintf(buf, "\t\tcr := t.childRanges[t.nodes[child].childID]\n")
	fmt.Fprintf(buf, "\t\tfor i := cr.start; i < cr.end; i++ {\n")
	fmt.Fprintf(buf, "\t\t\tcid := t.children[i]\n")
	fmt.Fprintf(buf, "\t\t\tif t.Type(cid) != NodeType_Node {\n")
	fmt.Fprintf(buf, "\t\t\t\tcontinue\n")
	fmt.Fprintf(buf, "\t\t\t}\n")
	fmt.Fprintf(buf, "\t\t\tswitch t.NameID(cid) {\n")
	for _, nc := range children {
		fmt.Fprintf(buf, "\t\t\tcase _nameID_%s:\n", nc.ruleName)
		if nc.repeated {
			fmt.Fprintf(buf, "\t\t\t\tv._%s = append(v._%s, cid)\n",
				fieldName(nc.ruleName), fieldName(nc.ruleName))
		} else {
			fmt.Fprintf(buf, "\t\t\t\tif !v._has%s {\n", nc.ruleName)
			fmt.Fprintf(buf, "\t\t\t\t\tv._%s = cid\n", fieldName(nc.ruleName))
			fmt.Fprintf(buf, "\t\t\t\t\tv._has%s = true\n", nc.ruleName)
			fmt.Fprintf(buf, "\t\t\t\t}\n")
		}
	}
	fmt.Fprintf(buf, "\t\t\t}\n")
	fmt.Fprintf(buf, "\t\t}\n")
	fmt.Fprintf(buf, "\t} else if t.Type(child) == NodeType_Node {\n")
	fmt.Fprintf(buf, "\t\tswitch t.NameID(child) {\n")
	for _, nc := range children {
		fmt.Fprintf(buf, "\t\tcase _nameID_%s:\n", nc.ruleName)
		if nc.repeated {
			fmt.Fprintf(buf, "\t\t\tv._%s = append(v._%s, child)\n",
				fieldName(nc.ruleName), fieldName(nc.ruleName))
		} else {
			fmt.Fprintf(buf, "\t\t\tv._%s = child\n", fieldName(nc.ruleName))
			fmt.Fprintf(buf, "\t\t\tv._has%s = true\n", nc.ruleName)
		}
	}
	fmt.Fprintf(buf, "\t\t}\n")
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "\treturn v\n")
	fmt.Fprintf(buf, "}\n\n")

	// --- accessors ---
	for _, nc := range children {
		if nc.repeated {
			emitRepeatedAccessor(buf, viewName, nc, rules)
		} else if nc.rule.Kind == RuleLeaf {
			emitLeafAccessor(buf, viewName, nc)
		} else {
			emitSingleAccessor(buf, viewName, nc, rules)
		}
	}
}

func emitLeafAccessor(buf *strings.Builder, viewName string, nc namedChild) {
	fmt.Fprintf(buf, "// %s returns the %s text.\n", nc.ruleName, nc.ruleName)
	fmt.Fprintf(buf, "func (v %s) %s() string {\n", viewName, nc.ruleName)
	fmt.Fprintf(buf, "\tif !v._has%s {\n", nc.ruleName)
	fmt.Fprintf(buf, "\t\treturn \"\"\n")
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "\treturn v.t.UnsafeText(v._%s)\n", fieldName(nc.ruleName))
	fmt.Fprintf(buf, "}\n\n")
}

func emitSingleAccessor(buf *strings.Builder, viewName string, nc namedChild, rules map[string]RuleInfo) {
	childViewName := nc.ruleName + "View"
	isSeq := nc.rule.Kind == RuleSequence
	fmt.Fprintf(buf, "// %s returns a view over the %s child.\n", nc.ruleName, nc.ruleName)
	fmt.Fprintf(buf, "func (v %s) %s() (%s, bool) {\n", viewName, nc.ruleName, childViewName)
	fmt.Fprintf(buf, "\tif !v._has%s {\n", nc.ruleName)
	fmt.Fprintf(buf, "\t\treturn %s{}, false\n", childViewName)
	fmt.Fprintf(buf, "\t}\n")
	if isSeq {
		fmt.Fprintf(buf, "\treturn new%s(v.t, v._%s), true\n", childViewName, fieldName(nc.ruleName))
	} else {
		fmt.Fprintf(buf, "\treturn %s{t: v.t, id: v._%s}, true\n", childViewName, fieldName(nc.ruleName))
	}
	fmt.Fprintf(buf, "}\n\n")
}

func emitRepeatedAccessor(buf *strings.Builder, viewName string, nc namedChild, rules map[string]RuleInfo) {
	childViewName := nc.ruleName + "View"
	isSeq := nc.rule.Kind == RuleSequence

	if nc.rule.Kind == RuleLeaf {
		fmt.Fprintf(buf, "// %sCount returns the number of %s children.\n", nc.ruleName, nc.ruleName)
		fmt.Fprintf(buf, "func (v %s) %sCount() int {\n", viewName, nc.ruleName)
		fmt.Fprintf(buf, "\treturn len(v._%s)\n", fieldName(nc.ruleName))
		fmt.Fprintf(buf, "}\n\n")

		fmt.Fprintf(buf, "// %sAt returns the text of the i-th %s child.\n", nc.ruleName, nc.ruleName)
		fmt.Fprintf(buf, "func (v %s) %sAt(i int) string {\n", viewName, nc.ruleName)
		fmt.Fprintf(buf, "\treturn v.t.UnsafeText(v._%s[i])\n", fieldName(nc.ruleName))
		fmt.Fprintf(buf, "}\n\n")
	} else {
		fmt.Fprintf(buf, "// %sCount returns the number of %s children.\n", nc.ruleName, nc.ruleName)
		fmt.Fprintf(buf, "func (v %s) %sCount() int {\n", viewName, nc.ruleName)
		fmt.Fprintf(buf, "\treturn len(v._%s)\n", fieldName(nc.ruleName))
		fmt.Fprintf(buf, "}\n\n")

		fmt.Fprintf(buf, "// %sAt returns a view over the i-th %s child.\n", nc.ruleName, nc.ruleName)
		fmt.Fprintf(buf, "func (v %s) %sAt(i int) %s {\n", viewName, nc.ruleName, childViewName)
		if isSeq {
			fmt.Fprintf(buf, "\treturn new%s(v.t, v._%s[i])\n", childViewName, fieldName(nc.ruleName))
		} else {
			fmt.Fprintf(buf, "\treturn %s{t: v.t, id: v._%s[i]}\n", childViewName, fieldName(nc.ruleName))
		}
		fmt.Fprintf(buf, "}\n\n")
	}
}

// emitChoiceView generates a thin wrapper. Choices inspect their single child
// on each accessor call (O(1) — just one Child() + nameID check).
func emitChoiceView(buf *strings.Builder, ri RuleInfo, rules map[string]RuleInfo) {
	viewName := ri.Name + "View"

	fmt.Fprintf(buf, "// %s is a read-only view over a %s node.\n", viewName, ri.Name)
	fmt.Fprintf(buf, "type %s struct {\n", viewName)
	fmt.Fprintf(buf, "\tt *tree\n")
	fmt.Fprintf(buf, "\tid NodeID\n")
	fmt.Fprintf(buf, "}\n\n")
	emitTextMethod(buf, viewName)

	for _, choice := range ri.Choices {
		if choice == "" {
			continue
		}
		childRule, ok := rules[choice]
		if !ok {
			continue
		}
		isSeq := childRule.Kind == RuleSequence

		if childRule.Kind == RuleLeaf {
			fmt.Fprintf(buf, "// %s returns the %s text if this alternative matched.\n", choice, choice)
			fmt.Fprintf(buf, "func (v %s) %s() (string, bool) {\n", viewName, choice)
			fmt.Fprintf(buf, "\tchild, ok := v.t.Child(v.id)\n")
			fmt.Fprintf(buf, "\tif !ok || !v.t.IsNamed(child, _nameID_%s) {\n", choice)
			fmt.Fprintf(buf, "\t\treturn \"\", false\n")
			fmt.Fprintf(buf, "\t}\n")
			fmt.Fprintf(buf, "\treturn v.t.UnsafeText(child), true\n")
			fmt.Fprintf(buf, "}\n\n")
		} else {
			childViewName := choice + "View"
			fmt.Fprintf(buf, "// %s returns a %s if this alternative matched.\n", choice, childViewName)
			fmt.Fprintf(buf, "func (v %s) %s() (%s, bool) {\n", viewName, choice, childViewName)
			fmt.Fprintf(buf, "\tchild, ok := v.t.Child(v.id)\n")
			fmt.Fprintf(buf, "\tif !ok || !v.t.IsNamed(child, _nameID_%s) {\n", choice)
			fmt.Fprintf(buf, "\t\treturn %s{}, false\n", childViewName)
			fmt.Fprintf(buf, "\t}\n")
			if isSeq {
				fmt.Fprintf(buf, "\treturn new%s(v.t, child), true\n", childViewName)
			} else {
				fmt.Fprintf(buf, "\treturn %s{t: v.t, id: child}, true\n", childViewName)
			}
			fmt.Fprintf(buf, "}\n\n")
		}
	}
}

func emitRepeatView(buf *strings.Builder, ri RuleInfo, rules map[string]RuleInfo) {
	viewName := ri.Name + "View"

	fmt.Fprintf(buf, "// %s is a read-only view over a %s node.\n", viewName, ri.Name)
	fmt.Fprintf(buf, "type %s struct {\n", viewName)
	fmt.Fprintf(buf, "\tt *tree\n")
	fmt.Fprintf(buf, "\tid NodeID\n")
	fmt.Fprintf(buf, "}\n\n")
	emitTextMethod(buf, viewName)

	if ri.Inner == "" {
		return
	}
	innerRule, ok := rules[ri.Inner]
	if !ok {
		return
	}

	nc := namedChild{ruleName: ri.Inner, rule: innerRule, repeated: true}
	// For repeat views, emit a Visit-style accessor that iterates direct
	// sequence children without pre-resolving into a slice.
	childViewName := nc.ruleName + "View"
	isSeq := nc.rule.Kind == RuleSequence

	if nc.rule.Kind == RuleLeaf {
		fmt.Fprintf(buf, "// Visit%s calls fn for each %s child text.\n", nc.ruleName, nc.ruleName)
		fmt.Fprintf(buf, "func (v %s) Visit%s(fn func(string) bool) {\n", viewName, nc.ruleName)
	} else {
		fmt.Fprintf(buf, "// Visit%s calls fn for each %s child view.\n", nc.ruleName, nc.ruleName)
		fmt.Fprintf(buf, "func (v %s) Visit%s(fn func(%s) bool) {\n", viewName, nc.ruleName, childViewName)
	}
	fmt.Fprintf(buf, "\tchild, ok := v.t.Child(v.id)\n")
	fmt.Fprintf(buf, "\tif !ok {\n")
	fmt.Fprintf(buf, "\t\treturn\n")
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "\tif v.t.Type(child) != NodeType_Sequence {\n")
	fmt.Fprintf(buf, "\t\treturn\n")
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "\tcr := v.t.childRanges[v.t.nodes[child].childID]\n")
	fmt.Fprintf(buf, "\tfor i := cr.start; i < cr.end; i++ {\n")
	fmt.Fprintf(buf, "\t\tcid := v.t.children[i]\n")
	fmt.Fprintf(buf, "\t\tif v.t.IsNamed(cid, _nameID_%s) {\n", nc.ruleName)
	if nc.rule.Kind == RuleLeaf {
		fmt.Fprintf(buf, "\t\t\tif !fn(v.t.UnsafeText(cid)) {\n")
	} else if isSeq {
		fmt.Fprintf(buf, "\t\t\tif !fn(new%s(v.t, cid)) {\n", childViewName)
	} else {
		fmt.Fprintf(buf, "\t\t\tif !fn(%s{t: v.t, id: cid}) {\n", childViewName)
	}
	fmt.Fprintf(buf, "\t\t\t\treturn\n")
	fmt.Fprintf(buf, "\t\t\t}\n")
	fmt.Fprintf(buf, "\t\t}\n")
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "}\n\n")
}

func emitAliasView(buf *strings.Builder, ri RuleInfo, rules map[string]RuleInfo) {
	viewName := ri.Name + "View"

	fmt.Fprintf(buf, "// %s is a read-only view over a %s node.\n", viewName, ri.Name)
	fmt.Fprintf(buf, "type %s struct {\n", viewName)
	fmt.Fprintf(buf, "\tt *tree\n")
	fmt.Fprintf(buf, "\tid NodeID\n")
	fmt.Fprintf(buf, "}\n\n")
	emitTextMethod(buf, viewName)

	if ri.Inner == "" {
		return
	}
	innerRule, ok := rules[ri.Inner]
	if !ok {
		return
	}
	if innerRule.Kind == RuleLeaf {
		return
	}

	childViewName := ri.Inner + "View"
	isSeq := innerRule.Kind == RuleSequence
	fmt.Fprintf(buf, "// %s returns a view over the aliased %s rule.\n", ri.Inner, ri.Inner)
	fmt.Fprintf(buf, "func (v %s) %s() (%s, bool) {\n", viewName, ri.Inner, childViewName)
	fmt.Fprintf(buf, "\tchild, ok := v.t.Child(v.id)\n")
	fmt.Fprintf(buf, "\tif !ok {\n")
	fmt.Fprintf(buf, "\t\treturn %s{}, false\n", childViewName)
	fmt.Fprintf(buf, "\t}\n")
	if isSeq {
		fmt.Fprintf(buf, "\treturn new%s(v.t, child), true\n", childViewName)
	} else {
		fmt.Fprintf(buf, "\treturn %s{t: v.t, id: child}, true\n", childViewName)
	}
	fmt.Fprintf(buf, "}\n\n")
}

func emitOptionalView(buf *strings.Builder, ri RuleInfo, rules map[string]RuleInfo) {
	emitAliasView(buf, ri, rules)
}

// isRepeatedChild checks if a rule name appears more than once in a
// sequence's classified children.
func isRepeatedChild(ri RuleInfo, ruleName string) bool {
	count := 0
	for _, c := range ri.Children {
		if c.RuleName == ruleName {
			count++
		}
	}
	return count > 1
}

// fieldName returns a lowercase version of a rule name for use as a struct
// field. We prepend underscore to avoid collisions with methods.
func fieldName(ruleName string) string {
	if len(ruleName) == 0 {
		return ""
	}
	// lowercase first character
	return strings.ToLower(ruleName[:1]) + ruleName[1:]
}
