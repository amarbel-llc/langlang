package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:generate go run ../../cmd/langlang -grammar ./orgmode.peg -output-language go -output-path ./orgmode.go -disable-spaces -disable-inline-defs=false

// parseDoc parses input as a Document and returns the tree + root node id.
func parseDoc(t *testing.T, input string) (Tree, NodeID) {
	t.Helper()
	p := NewParser()
	p.SetInput([]byte(input))
	tree, err := p.ParseDocument()
	require.NoError(t, err, "ParseDocument failed for input %q", input)
	root, ok := tree.Root()
	require.True(t, ok, "tree has no root for input %q", input)
	return tree, root
}

// namedChildren returns the names of named (NodeType_Node) descendants reached
// by descending only through Sequence nodes — i.e. the immediate "grammar
// children" of id, ignoring sequence wrappers and string terminals. Pure
// alternation wrappers (currently only "Element") are transparently unwrapped
// so callers see the underlying production (Headline, Paragraph, ...) instead.
func namedChildren(tree Tree, id NodeID) []string {
	var out []string
	var walk func(NodeID)
	walk = func(n NodeID) {
		if tree.Type(n) == NodeType_Node {
			if tree.Name(n) == "Element" {
				// transparent: descend into the chosen alternative
				for c := range tree.IterDirectChildren(n) {
					walk(c)
				}
				return
			}
			out = append(out, tree.Name(n))
			return
		}
		for c := range tree.IterDirectChildren(n) {
			walk(c)
		}
	}
	for c := range tree.IterDirectChildren(id) {
		walk(c)
	}
	return out
}

// findFirst returns the first descendant (depth-first) whose name matches.
func findFirst(tree Tree, id NodeID, name string) (NodeID, bool) {
	var found NodeID
	ok := false
	tree.Visit(id, func(n NodeID) bool {
		if tree.Type(n) == NodeType_Node && tree.Name(n) == name {
			found = n
			ok = true
			return false
		}
		return true
	})
	return found, ok
}

func TestSingleHeadline(t *testing.T) {
	tree, root := parseDoc(t, "* Hello world\n")
	require.Equal(t, "Document", tree.Name(root))
	require.Equal(t, []string{"Headline"}, namedChildren(tree, root))

	hl, ok := findFirst(tree, root, "Headline")
	require.True(t, ok)
	require.Equal(t, []string{"Stars", "Title", "EOL"}, namedChildren(tree, hl))

	stars, _ := findFirst(tree, hl, "Stars")
	require.Equal(t, "*", tree.Text(stars))

	title, _ := findFirst(tree, hl, "Title")
	require.Equal(t, "Hello world", tree.Text(title))
}

func TestNestedHeadlines(t *testing.T) {
	tree, root := parseDoc(t, "* Top\n** Mid\n*** Deep\n")
	require.Equal(t, []string{"Headline", "Headline", "Headline"}, namedChildren(tree, root))

	var depths []int
	tree.Visit(root, func(n NodeID) bool {
		if tree.Type(n) == NodeType_Node && tree.Name(n) == "Stars" {
			depths = append(depths, len(tree.Text(n)))
		}
		return true
	})
	require.Equal(t, []int{1, 2, 3}, depths)
}

func TestParagraph(t *testing.T) {
	tree, root := parseDoc(t, "Just a line of text.\nAnd another.\n")
	require.Equal(t, []string{"Paragraph"}, namedChildren(tree, root))

	para, _ := findFirst(tree, root, "Paragraph")
	lines := namedChildren(tree, para)
	require.Equal(t, []string{"ParaLine", "ParaLine"}, lines)
}

func TestHeadlineThenParagraph(t *testing.T) {
	tree, root := parseDoc(t, "* Title\nBody text here.\n")
	require.Equal(t, []string{"Headline", "Paragraph"}, namedChildren(tree, root))
}

func TestSourceBlockOpaqueBody(t *testing.T) {
	src := "#+BEGIN_SRC go\n* not a headline\n- not a list\n#+END_SRC\n"
	tree, root := parseDoc(t, src)
	require.Equal(t, []string{"Block"}, namedChildren(tree, root))

	block, _ := findFirst(tree, root, "Block")
	require.Equal(t, []string{"BlockBegin", "BlockBody", "BlockEnd"}, namedChildren(tree, block))

	body, _ := findFirst(tree, block, "BlockBody")
	require.Equal(t, "* not a headline\n- not a list\n", tree.Text(body))

	// The body's opaque content must NOT contain a parsed Headline.
	_, hasHeadline := findFirst(tree, body, "Headline")
	require.False(t, hasHeadline, "body should be opaque, no Headline inside")
}

func TestBlockWithArgs(t *testing.T) {
	tree, root := parseDoc(t, "#+BEGIN_SRC go :tangle foo.go\nx := 1\n#+END_SRC\n")
	block, _ := findFirst(tree, root, "Block")

	begin, _ := findFirst(tree, block, "BlockBegin")
	name, _ := findFirst(tree, begin, "BlockName")
	require.Equal(t, "SRC", tree.Text(name))

	args, _ := findFirst(tree, begin, "BlockArgs")
	require.Equal(t, " go :tangle foo.go", tree.Text(args))
}

func TestBulletList(t *testing.T) {
	tree, root := parseDoc(t, "- first\n- second\n+ third\n")
	require.Equal(t, []string{"List"}, namedChildren(tree, root))

	list, _ := findFirst(tree, root, "List")
	require.Equal(t, []string{"ListItem", "ListItem", "ListItem"}, namedChildren(tree, list))

	var bullets, bodies []string
	tree.Visit(list, func(n NodeID) bool {
		if tree.Type(n) != NodeType_Node {
			return true
		}
		switch tree.Name(n) {
		case "Bullet":
			bullets = append(bullets, tree.Text(n))
		case "ItemBody":
			bodies = append(bodies, tree.Text(n))
		}
		return true
	})
	require.Equal(t, []string{"-", "-", "+"}, bullets)
	require.Equal(t, []string{"first", "second", "third"}, bodies)
}

func TestBlankLineSeparator(t *testing.T) {
	tree, root := parseDoc(t, "* H\n\nBody\n")
	require.Equal(t, []string{"Headline", "BlankLine", "Paragraph"}, namedChildren(tree, root))
}

func TestMixedDocument(t *testing.T) {
	input := strings.Join([]string{
		"* Heading one",
		"Some intro text.",
		"",
		"** Subheading",
		"- item a",
		"- item b",
		"",
		"#+BEGIN_EXAMPLE",
		"raw stuff",
		"#+END_EXAMPLE",
		"",
		"Trailing paragraph.",
		"",
	}, "\n")

	tree, root := parseDoc(t, input)
	got := namedChildren(tree, root)
	want := []string{
		"Headline",
		"Paragraph",
		"BlankLine",
		"Headline",
		"List",
		"BlankLine",
		"Block",
		"BlankLine",
		"Paragraph",
	}
	require.Equal(t, want, got)
}

func TestHeadlineWithTodoKeyword(t *testing.T) {
	tree, root := parseDoc(t, "* TODO Buy milk\n* DONE Walk dog\n* NEXT Read book\n* Plain heading\n")
	require.Equal(t, []string{"Headline", "Headline", "Headline", "Headline"}, namedChildren(tree, root))

	var todos []string
	tree.Visit(root, func(n NodeID) bool {
		if tree.Type(n) == NodeType_Node && tree.Name(n) == "TodoKeyword" {
			// TodoKeyword swallows the trailing space; trim for comparison.
			todos = append(todos, strings.TrimSpace(tree.Text(n)))
		}
		return true
	})
	require.Equal(t, []string{"TODO", "DONE", "NEXT"}, todos)

	// The plain heading must NOT have a TodoKeyword child.
	var titles []string
	tree.Visit(root, func(n NodeID) bool {
		if tree.Type(n) == NodeType_Node && tree.Name(n) == "Title" {
			titles = append(titles, tree.Text(n))
		}
		return true
	})
	require.Equal(t, []string{"Buy milk", "Walk dog", "Read book", "Plain heading"}, titles)
}

func TestPropertiesDrawer(t *testing.T) {
	input := "* Heading\n:PROPERTIES:\n:ID:       abc-123\n:CREATED:  [2026-04-07]\n:END:\nBody text.\n"
	tree, root := parseDoc(t, input)
	require.Equal(t, []string{"Headline", "PropertiesDrawer", "Paragraph"}, namedChildren(tree, root))

	drawer, _ := findFirst(tree, root, "PropertiesDrawer")
	require.Equal(t, []string{"PropertiesOpen", "DrawerBody", "PropertiesClose"}, namedChildren(tree, drawer))

	body, _ := findFirst(tree, drawer, "DrawerBody")
	require.Equal(t, ":ID:       abc-123\n:CREATED:  [2026-04-07]\n", tree.Text(body))

	// The drawer body is opaque — no Headline/ParaLine/etc. parsed inside it.
	for _, name := range []string{"Headline", "ParaLine", "ListItem"} {
		_, found := findFirst(tree, body, name)
		require.False(t, found, "drawer body should be opaque, found %q inside", name)
	}
}

func TestEmptyPropertiesDrawer(t *testing.T) {
	tree, root := parseDoc(t, "* H\n:PROPERTIES:\n:END:\n")
	require.Equal(t, []string{"Headline", "PropertiesDrawer"}, namedChildren(tree, root))

	drawer, _ := findFirst(tree, root, "PropertiesDrawer")
	// When DrawerBody matches zero characters, langlang elides the named
	// node from the tree. Either a present-but-empty node or no node at
	// all is acceptable; both express "no body content".
	if body, ok := findFirst(tree, drawer, "DrawerBody"); ok {
		require.Equal(t, "", tree.Text(body))
	}
}

func TestParagraphNotSwallowingDrawer(t *testing.T) {
	// Without the !PropertiesOpen lookahead in ParaLine, a paragraph could
	// greedily consume the :PROPERTIES: line as text.
	tree, root := parseDoc(t, "* H\nIntro line.\n:PROPERTIES:\n:K: v\n:END:\n")
	require.Equal(t, []string{"Headline", "Paragraph", "PropertiesDrawer"}, namedChildren(tree, root))
}

func TestNoTrailingNewline(t *testing.T) {
	// EOL accepts !. so the last line need not end in '\n'.
	tree, root := parseDoc(t, "* Last")
	require.Equal(t, []string{"Headline"}, namedChildren(tree, root))
}
