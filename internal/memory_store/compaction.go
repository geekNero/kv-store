package memorystore

type SSTEntry struct {
	key       string
	value     string
	operation string
	sstID     int
}

type MinMergeHeap []*SSTEntry

func (h MinMergeHeap) Len() int {
	return len(h)
}

func (h *MinMergeHeap) Push(x any) {
	*h = append(*h, x.(*SSTEntry))
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
	if h[i].key != h[j].key {
		return h[i].key < h[j].key
	}

	return h[i].sstID > h[j].sstID
}

func (h MinMergeHeap) Peak() *SSTEntry {
	if h.Len() == 0 {
		return nil
	}
	return h[0]
}
