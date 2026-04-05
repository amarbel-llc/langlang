package jsonextract

import "fmt"

// ExtractJSONValueArena extracts a JSONValue from the tree, allocating
// from the per-type arenas. Returns a pointer into a.Values.
// The arenas must be pre-allocated via Alloc() with counts from
// CountJSONNodes() to ensure pointer stability.
func ExtractJSONValueArena(t *tree, id NodeID, a *JSONArenas) (*JSONValue, error) {
	idx := len(a.Values)
	a.Values = append(a.Values, JSONValue{})

	child, ok := t.Child(id)
	if !ok {
		return &a.Values[idx], fmt.Errorf("JSONValue: no child")
	}

	// All nested calls may grow arena slices, so we access a.Values[idx]
	// only after the call returns (re-derive, not cached pointer).
	switch {
	case t.IsNamed(child, _nameID_Object):
		val, err := ExtractJSONObjectArena(t, child, a)
		if err != nil {
			return &a.Values[idx], err
		}
		a.Values[idx].Object = val
	case t.IsNamed(child, _nameID_Array):
		val, err := ExtractJSONArrayArena(t, child, a)
		if err != nil {
			return &a.Values[idx], err
		}
		a.Values[idx].Array = val
	case t.IsNamed(child, _nameID_String):
		a.Values[idx].String = a.allocString(t.Text(child))
	case t.IsNamed(child, _nameID_Number):
		a.Values[idx].Number = a.allocString(t.Text(child))
	case t.Type(child) == NodeType_String:
		// literal alternative (e.g., 'true', 'false', 'null')
	}
	return &a.Values[idx], nil
}

// ExtractJSONObjectArena extracts a JSONObject, allocating from arenas.
// Members are stored in a sub-slice of MemberBuf (no per-object allocation).
//
// Because nested object extraction also appends to MemberBuf, we can't
// use a simple start:end range. Instead we reserve slots for this object's
// members first (using direct child count from the pre-count), then fill
// them in order. Direct children are visited without recursion (return false),
// and each member extraction may recurse into nested objects — but those
// append to MemberBuf beyond our reserved range.
func ExtractJSONObjectArena(t *tree, id NodeID, a *JSONArenas) (*JSONObject, error) {
	idx := len(a.Objects)
	a.Objects = append(a.Objects, JSONObject{})

	// Count member children (Visit recurses through Sequence wrappers
	// but stops at Member nodes to avoid counting nested objects' members).
	var memberCount int
	t.Visit(id, func(cid NodeID) bool {
		if cid == id {
			return true
		}
		if t.Type(cid) != NodeType_Node {
			return true
		}
		if t.NameID(cid) == _nameID_Member {
			memberCount++
			return false // don't recurse into member's children
		}
		return true
	})

	if memberCount > 0 {
		// Reserve contiguous slots in MemberBuf for this object's members.
		memberStart := len(a.MemberBuf)
		a.MemberBuf = append(a.MemberBuf, make([]JSONMember, memberCount)...)
		members := a.MemberBuf[memberStart : memberStart+memberCount]

		var i int
		t.Visit(id, func(cid NodeID) bool {
			if cid == id {
				return true
			}
			if t.Type(cid) != NodeType_Node {
				return true
			}
			if t.NameID(cid) == _nameID_Member {
				extractJSONMemberArenaInto(t, cid, a, &members[i])
				i++
				return false // don't recurse into member's children
			}
			return true
		})
		// Re-derive out — nested extraction may have grown a.Objects,
		// invalidating the original pointer.
		a.Objects[idx].Members = members[:i]
	}
	return &a.Objects[idx], nil
}

// extractJSONMemberArenaInto fills a pre-allocated JSONMember in place.
func extractJSONMemberArenaInto(t *tree, id NodeID, a *JSONArenas, out *JSONMember) {
	t.Visit(id, func(cid NodeID) bool {
		if cid == id {
			return true
		}
		if t.Type(cid) != NodeType_Node {
			return true
		}
		switch t.NameID(cid) {
		case _nameID_String:
			out.Key = t.Text(cid)
			return false
		case _nameID_Value:
			val, err := ExtractJSONValueArena(t, cid, a)
			if err == nil {
				out.Value = *val
			}
			return false
		}
		return true
	})
}

// ExtractJSONArrayArena extracts a JSONArray, allocating from arenas.
// Items are stored in a sub-slice of ItemBuf (no per-array allocation).
// Same reservation strategy as ExtractJSONObjectArena.
func ExtractJSONArrayArena(t *tree, id NodeID, a *JSONArenas) (*JSONArray, error) {
	idx := len(a.Arrays)
	a.Arrays = append(a.Arrays, JSONArray{})

	// Count Value children (same pattern as Object member counting).
	var itemCount int
	t.Visit(id, func(cid NodeID) bool {
		if cid == id {
			return true
		}
		if t.Type(cid) != NodeType_Node {
			return true
		}
		if t.NameID(cid) == _nameID_Value {
			itemCount++
			return false
		}
		return true
	})

	if itemCount > 0 {
		itemStart := len(a.ItemBuf)
		a.ItemBuf = append(a.ItemBuf, make([]JSONValue, itemCount)...)
		items := a.ItemBuf[itemStart : itemStart+itemCount]

		var i int
		t.Visit(id, func(cid NodeID) bool {
			if cid == id {
				return true
			}
			if t.Type(cid) != NodeType_Node {
				return true
			}
			if t.NameID(cid) == _nameID_Value {
				val, err := ExtractJSONValueArena(t, cid, a)
				if err == nil {
					items[i] = *val
					i++
				}
				return false
			}
			return true
		})
		// Re-derive — nested extraction may have grown a.Arrays.
		a.Arrays[idx].Items = items[:i]
	}
	return &a.Arrays[idx], nil
}
