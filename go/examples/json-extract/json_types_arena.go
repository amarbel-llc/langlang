package jsonextract

// JSONArenas holds per-type arena slices for allocation-free extraction.
// Pre-allocate with Alloc() using counts from CountJSONNodes() to ensure
// pointer stability — all pointers into arena slices remain valid because
// no append exceeds capacity.
//
// MemberBuf and ItemBuf are flat backing arrays for Object.Members and
// Array.Items slices. Each extraction sub-slices into these buffers
// instead of allocating individual slices.
//
// StringBuf is a flat backing array for string pointer fields (String,
// Number in JSONValue). Extraction appends the string value and takes
// &StringBuf[idx] instead of heap-allocating a *string.
//
// NOTE: StringBuf uses the fact that *string points into the arena
// backing array. This is safe as long as the arena isn't reallocated
// (guaranteed by pre-sizing). A future optimization could use
// unsafe.Pointer to alias directly into the parse tree's input buffer,
// eliminating even the string copy — flagged but not implemented here.
type JSONArenas struct {
	Values    []JSONValue
	Objects   []JSONObject
	Members   []JSONMember
	Arrays    []JSONArray
	MemberBuf []JSONMember // flat backing for all Object.Members
	ItemBuf   []JSONValue  // flat backing for all Array.Items
	StringBuf []string     // flat backing for *string fields
}

// JSONNodeCounts holds pre-counted node totals from CountJSONNodes.
type JSONNodeCounts struct {
	Values  int
	Objects int
	Members int
	Arrays  int
	Strings int // String + Number leaf nodes (need *string pointers)
}

// CountJSONNodes walks the tree and counts nodes per type.
func CountJSONNodes(t *tree, root NodeID) JSONNodeCounts {
	var c JSONNodeCounts
	t.Visit(root, func(id NodeID) bool {
		if t.Type(id) != NodeType_Node {
			return true
		}
		switch t.NameID(id) {
		case _nameID_Value:
			c.Values++
		case _nameID_Object:
			c.Objects++
		case _nameID_Member:
			c.Members++
		case _nameID_Array:
			c.Arrays++
		case _nameID_String:
			c.Strings++
		case _nameID_Number:
			c.Strings++
		}
		return true
	})
	return c
}

// Alloc pre-allocates all arena slices to exact capacity.
func (a *JSONArenas) Alloc(c JSONNodeCounts) {
	a.Values = make([]JSONValue, 0, c.Values)
	a.Objects = make([]JSONObject, 0, c.Objects)
	a.Members = make([]JSONMember, 0, c.Members)
	a.Arrays = make([]JSONArray, 0, c.Arrays)
	a.MemberBuf = make([]JSONMember, 0, c.Members)
	a.ItemBuf = make([]JSONValue, 0, c.Values) // upper bound: all values could be array items
	a.StringBuf = make([]string, 0, c.Strings)
}

// Reset clears all arenas for reuse without releasing memory.
func (a *JSONArenas) Reset() {
	a.Values = a.Values[:0]
	a.Objects = a.Objects[:0]
	a.Members = a.Members[:0]
	a.Arrays = a.Arrays[:0]
	a.MemberBuf = a.MemberBuf[:0]
	a.ItemBuf = a.ItemBuf[:0]
	a.StringBuf = a.StringBuf[:0]
}

// allocString appends s to StringBuf and returns a pointer into the buffer.
func (a *JSONArenas) allocString(s string) *string {
	idx := len(a.StringBuf)
	a.StringBuf = append(a.StringBuf, s)
	return &a.StringBuf[idx]
}
