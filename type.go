package skiplist

import (
	"math/rand"
	"sync"
)

type SkippedSequenceEntry struct {
	Start     uint64
	End       uint64
	Timestamp int64
}

func (s *SkippedSequenceEntry) GetNumSequencesInEntry() int64 {
	if s.Start == s.End {
		return 1
	}
	return int64(s.End - s.Start + 1)
}

type elementNode struct {
	next []*Element
}

type Element struct {
	elementNode
	key SkippedSequenceEntry
}

// Key allows retrieval of the key for a given Element
func (e *Element) Key() SkippedSequenceEntry {
	return e.key
}

// Next returns the following Element or nil if we're at the end of the list.
// Only operates on the bottom level of the skip list (a fully linked list).
func (element *Element) Next() *Element {
	return element.next[0]
}

func (element *Element) IsRange() bool {
	return element.key.Start != element.key.End
}

func (element *Element) IsWithinRange(elem SkippedSequenceEntry) bool {
	return element.key.Start <= elem.Start && element.key.End >= elem.End
}

type SkipList struct {
	elementNode
	NumSequencesInList int64
	maxLevel           int
	Length             int
	randSource         rand.Source
	probability        float64
	probTable          []float64
	mutex              sync.RWMutex
	backElem           *Element
	prevNodesCache     []*elementNode
}

func (list *SkipList) GetLength() int {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.Length
}

func (list *SkipList) GetLastElement() *Element {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.backElem
}

func (list *SkipList) GetNumSequencesInList() int64 {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.NumSequencesInList
}
