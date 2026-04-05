package junction

import "bytes"

// ScanJunctions finds all structural delimiters in input, respecting
// quoting contexts. Returns hits in input order with depth tracking.
//
// Single-byte junctions use a [256]uint8 LUT for O(1) classification.
// Multi-byte sequences use a trigger LUT on their first byte; when
// triggered, candidates are checked by lookahead (longest match wins).
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

	// Build sequence trigger lookup: byte -> true if any sequence starts
	// with this byte. Sort sequences by length descending so longest
	// match is checked first.
	var seqTrigger [256]bool
	seqs := sortSequencesByLength(spec.Sequences)
	for _, seq := range seqs {
		if len(seq.Pattern) > 0 {
			seqTrigger[seq.Pattern[0]] = true
		}
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

		// Try multi-byte sequences before single-byte (longest match wins).
		if seqTrigger[b] {
			if hit, n := matchSequence(input, i, seqs, depth); n > 0 {
				hits = append(hits, hit)
				switch hit.Kind {
				case JunctionOpen:
					depth++
				case JunctionClose:
					depth--
				}
				i += n - 1 // -1 because the loop increments
				continue
			}
		}

		if jk := junctionLUT[b]; jk != 0 {
			kind := JunctionKind(jk - 1)
			hit := JunctionHit{
				Pos:   int32(i),
				Depth: depth,
				Kind:  kind,
				Byte:  b,
				Len:   1,
			}
			switch kind {
			case JunctionOpen:
				hits = append(hits, hit)
				depth++
			case JunctionClose:
				depth--
				hit.Depth = depth
				hits = append(hits, hit)
			case JunctionSeparator:
				hits = append(hits, hit)
			}
		}
	}

	return hits
}

// matchSequence checks all candidate sequences against input[pos:].
// Sequences are pre-sorted longest-first, so the first match is the
// longest. Returns the hit and the number of bytes consumed, or 0 if
// no sequence matched.
func matchSequence(input []byte, pos int, seqs []JunctionSequence, depth int16) (JunctionHit, int) {
	remaining := input[pos:]
	for _, seq := range seqs {
		if len(seq.Pattern) > len(remaining) {
			continue
		}
		if bytes.Equal(remaining[:len(seq.Pattern)], seq.Pattern) {
			d := depth
			if seq.Kind == JunctionClose {
				d--
			}
			return JunctionHit{
				Pos:   int32(pos),
				Depth: d,
				Kind:  seq.Kind,
				Byte:  seq.Pattern[0],
				Len:   int16(len(seq.Pattern)),
			}, len(seq.Pattern)
		}
	}
	return JunctionHit{}, 0
}

// sortSequencesByLength returns a copy sorted by pattern length descending
// (longest first) for greedy matching.
func sortSequencesByLength(seqs []JunctionSequence) []JunctionSequence {
	if len(seqs) == 0 {
		return nil
	}
	sorted := make([]JunctionSequence, len(seqs))
	copy(sorted, seqs)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j].Pattern) > len(sorted[j-1].Pattern); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}
