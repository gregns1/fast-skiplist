package skiplist

import (
	"math/rand"
	"sync"
)

type SkippedSequenceEntry struct {
	start     uint64
	end       uint64
	timestamp int64
}

type elementNode struct {
	next []*Element
}

type Element struct {
	elementNode
	key   uint64
	value interface{}
}

// Key allows retrieval of the key for a given Element
func (e *Element) Key() uint64 {
	return e.key
}

// Value allows retrieval of the value for a given Element
func (e *Element) Value() interface{} {
	return e.value
}

// Next returns the following Element or nil if we're at the end of the list.
// Only operates on the bottom level of the skip list (a fully linked list).
func (element *Element) Next() *Element {
	return element.next[0]
}

//
//func (element *Element) IsRange() bool {
//	return element.value.start != element.value.end
//}

type SkipList struct {
	elementNode
	maxLevel       int
	Length         int
	randSource     rand.Source
	probability    float64
	probTable      []float64
	mutex          sync.RWMutex
	prevNodesCache []*elementNode
}
