package junction

// ScanJunctions finds all structural delimiter bytes in input, respecting
// quoting contexts. Returns hits in input order with depth tracking.
func ScanJunctions(input []byte, spec ScannerSpec) []JunctionHit {
	// Build lookup table: byte -> JunctionKind+1 (0 means not a junction).
	var junctionLUT [256]uint8
	for _, jb := range spec.Junctions {
		junctionLUT[jb.Byte] = uint8(jb.Kind) + 1
	}

	// Build quoting lookup: byte -> index+1 into spec.Quoting (0 means not a delimiter).
	var quoteLUT [256]uint8
	for i, qc := range spec.Quoting {
		quoteLUT[qc.Delimiter] = uint8(i) + 1
	}

	var hits []JunctionHit
	var depth int16
	var inQuote bool
	var activeQuote QuotingContext

	for i := 0; i < len(input); i++ {
		b := input[i]

		if inQuote {
			if activeQuote.EscapePrefix != 0 && b == activeQuote.EscapePrefix {
				i++ // skip escaped character
				continue
			}
			if b == activeQuote.Delimiter {
				inQuote = false
			}
			continue
		}

		if qi := quoteLUT[b]; qi != 0 {
			inQuote = true
			activeQuote = spec.Quoting[qi-1]
			continue
		}

		if jk := junctionLUT[b]; jk != 0 {
			kind := JunctionKind(jk - 1)
			switch kind {
			case JunctionOpen:
				hits = append(hits, JunctionHit{
					Pos:  int32(i),
					Depth: depth,
					Kind: kind,
					Byte: b,
				})
				depth++
			case JunctionClose:
				depth--
				hits = append(hits, JunctionHit{
					Pos:  int32(i),
					Depth: depth,
					Kind: kind,
					Byte: b,
				})
			case JunctionSeparator:
				hits = append(hits, JunctionHit{
					Pos:  int32(i),
					Depth: depth,
					Kind: kind,
					Byte: b,
				})
			}
		}
	}

	return hits
}
