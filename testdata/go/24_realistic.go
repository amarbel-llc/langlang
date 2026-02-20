package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type WordCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewWordCounter() *WordCounter {
	return &WordCounter{
		counts: make(map[string]int),
	}
}

func (wc *WordCounter) Add(text string) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	words := strings.Fields(text)
	for _, w := range words {
		w = strings.ToLower(w)
		wc.counts[w]++
	}
}

func (wc *WordCounter) TopN(n int) []WordCount {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	result := make([]WordCount, 0, len(wc.counts))
	for word, count := range wc.counts {
		result = append(result, WordCount{Word: word, Count: count})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Word < result[j].Word
	})

	if n > len(result) {
		n = len(result)
	}
	return result[:n]
}

type WordCount struct {
	Word  string
	Count int
}

func main() {
	wc := NewWordCounter()

	texts := []string{
		"the quick brown fox jumps over the lazy dog",
		"the fox runs fast and the dog chases the fox",
	}

	var wg sync.WaitGroup
	for _, t := range texts {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			wc.Add(text)
		}(t)
	}
	wg.Wait()

	for _, entry := range wc.TopN(5) {
		fmt.Printf("%-10s %d\n", entry.Word, entry.Count)
	}
}
