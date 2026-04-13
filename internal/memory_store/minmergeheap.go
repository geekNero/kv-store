package memorystore

import "kv_store/internal/spec"

type HeapEntry struct {
	*spec.SSTEntry
	sstID int
	index int
}

type MinMergeHeap []*HeapEntry

func (h MinMergeHeap) Len() int {
	return len(h)
}

func (h *MinMergeHeap) Push(x any) {
	*h = append(*h, x.(*HeapEntry))
}

func (h *MinMergeHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

func (h MinMergeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h MinMergeHeap) Less(i, j int) bool {
	if h[i].Key != h[j].Key {
		return h[i].Key < h[j].Key
	}

	return h[i].sstID > h[j].sstID
}

func (h MinMergeHeap) Peek() *HeapEntry {
	if h.Len() == 0 {
		return nil
	}
	return h[0]
}

func compact(iterables []*fileIterator)
