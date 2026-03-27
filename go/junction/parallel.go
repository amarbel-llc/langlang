package junction

import (
	"runtime"
	"sync"
)

// ParseFunc parses a byte slice and returns an opaque result or an error.
// The caller is responsible for ensuring the result is safe to use after
// the parse function returns (e.g. by calling Tree.Copy() if needed).
type ParseFunc func(input []byte) (any, error)

// PartitionResult holds the parse result for a single partition.
type PartitionResult struct {
	Partition Partition
	Value     any
	Err       error
}

// ParsePartitions parses each partition in the given slice in parallel using
// the provided parse function. Each goroutine gets its own invocation of
// parseFn, so parseFn must be safe to call concurrently (typically by
// capturing a sync.Pool of parsers or creating a new parser per call).
//
// The input slice is the full original input; each partition's [Start, End)
// range indexes into it. Returns results in the same order as parts.
func ParsePartitions(input []byte, parts []Partition, parseFn ParseFunc) []PartitionResult {
	if len(parts) == 0 {
		return nil
	}

	results := make([]PartitionResult, len(parts))

	workers := runtime.GOMAXPROCS(0)
	if workers > len(parts) {
		workers = len(parts)
	}

	if workers <= 1 {
		for i, part := range parts {
			slice := input[part.Start:part.End]
			val, err := parseFn(slice)
			results[i] = PartitionResult{Partition: part, Value: val, Err: err}
		}
		return results
	}

	var wg sync.WaitGroup
	work := make(chan int, len(parts))

	for i := range parts {
		work <- i
	}
	close(work)

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := range work {
				part := parts[i]
				slice := input[part.Start:part.End]
				val, err := parseFn(slice)
				results[i] = PartitionResult{Partition: part, Value: val, Err: err}
			}
		}()
	}

	wg.Wait()
	return results
}
