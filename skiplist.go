package skiplist

import (
	"math"
	"math/rand"
	"time"
)

const (
	// Suitable for math.Floor(math.Pow(math.E, 18)) == 65659969 elements in list
	DefaultMaxLevel    int     = 18
	DefaultProbability float64 = 1 / math.E
)

// Front returns the head node of the list.
func (list *SkipList) Front() *Element {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.next[0]
}

// Front returns the head node of the list without acquiring mutex.
func (list *SkipList) _front() *Element {
	return list.next[0]
}

// Set inserts a value in the list with the specified key, ordered by the key.
// If the key exists, it updates the value in the existing node.
// Returns a pointer to the new element.
// Locking is optimistic and happens only after searching.
func (list *SkipList) Set(key SkippedSequenceEntry) *Element {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	var element *Element
	prevs := list.getPrevElementNodes(key)

	if element = prevs[0].next[0]; element != nil && element.IsWithinRange(key) {
		element.key = key
		return element
	}

	// if appending contiguous element to the back of the list, just extend the last element
	if list.backElem != nil && key.Start == list.backElem.key.End+1 {
		list.backElem.key.End = key.End
		list.backElem.key.Timestamp = key.Timestamp
		return list.backElem
	}

	element = &Element{
		elementNode: elementNode{
			next: make([]*Element, list.randLevel()),
		},
		key: key,
	}

	for i := range element.next {
		element.next[i] = prevs[i].next[i]
		prevs[i].next[i] = element
	}

	// update last item in list if item added was pushed to back
	if element.Next() == nil {
		list.backElem = element
	}

	// update stats
	list.NumSequencesInList += key.GetNumSequencesInEntry()

	list.Length++
	return element
}

// _set is a private function that sets an element in the list. Same functionality as Set but doesn't acquire
// mutex and won't increment sequences stats. This is because its only called when we're splitting ranges in the list
func (list *SkipList) _set(key SkippedSequenceEntry) *Element {
	var element *Element
	prevs := list.getPrevElementNodes(key)

	if element = prevs[0].next[0]; element != nil && element.IsWithinRange(key) {
		return element
	}

	element = &Element{
		elementNode: elementNode{
			next: make([]*Element, list.randLevel()),
		},
		key: key,
	}

	for i := range element.next {
		element.next[i] = prevs[i].next[i]
		prevs[i].next[i] = element
	}

	// update last item in list if item added was pushed to back
	if element.Next() == nil {
		list.backElem = element
	}

	list.Length++
	return element
}

// Get finds an element by key. It returns element pointer if found, nil if not found.
// Locking is optimistic and happens only after searching with a fast check for deletion after locking.
func (list *SkipList) Get(key SkippedSequenceEntry) *Element {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	var prev *elementNode = &list.elementNode
	var next *Element

	for i := list.maxLevel - 1; i >= 0; i-- {
		next = prev.next[i]

		for next != nil && key.Start > next.key.End {
			prev = &next.elementNode
			next = next.next[i]
		}
	}

	if next != nil && next.IsWithinRange(key) {
		return next
	}

	return nil
}

// Remove deletes an element from the list.
// Returns removed element pointer if found, nil if not found.
// Locking is optimistic and happens only after searching with a fast check on adjacent nodes after locking.
func (list *SkipList) Remove(key SkippedSequenceEntry) *Element {
	list.mutex.Lock()
	defer list.mutex.Unlock()
	prevs := list.getPrevElementNodes(key)

	if element := prevs[0].next[0]; element != nil && element.IsWithinRange(key) {
		if !element.IsRange() || (element.key.Start == key.Start && element.key.End == key.End) {
			// remove whole element
			for k, v := range element.next {
				prevs[k].next[k] = v
			}
			// update stats
			list.NumSequencesInList -= key.GetNumSequencesInEntry()
			list.Length--
			// return removed element
			return element
		}
		// subset of element to remove/split
		if key.Start == element.key.Start {
			element.key = SkippedSequenceEntry{Start: key.End + 1, End: element.key.End, Timestamp: element.key.Timestamp}
			list.NumSequencesInList -= key.GetNumSequencesInEntry()
			// return non nil elem as we have found item
			return &Element{}
		}
		if key.End == element.key.End {
			element.key = SkippedSequenceEntry{Start: element.key.Start, End: key.Start - 1, Timestamp: element.key.Timestamp}
			list.NumSequencesInList -= key.GetNumSequencesInEntry()
			// return non nil elem as we have found item
			return &Element{}
		}
		// need to split current element around incoming key
		newEntryKey := SkippedSequenceEntry{Start: key.End + 1, End: element.key.End, Timestamp: element.key.Timestamp}
		newCurrElemKey := SkippedSequenceEntry{Start: element.key.Start, End: key.Start - 1, Timestamp: element.key.Timestamp}
		element.key = newCurrElemKey
		// insert new element
		list._set(newEntryKey)
		// update stats
		list.NumSequencesInList -= key.GetNumSequencesInEntry()

		return &Element{}
	} else if element != nil {
		var removedSeqs bool
		for e := element; e != nil; e = e.Next() {
			if key.End < e.key.Start {
				break
			}
			var removeStartSeq uint64
			var removeEndSeq uint64
			if e.key.Start >= key.Start {
				removeStartSeq = e.key.Start
			} else if element.key.End >= key.Start {
				removeStartSeq = key.Start
			}

			if e.key.End <= key.End {
				removeEndSeq = e.key.End
			} else if e.key.End >= key.End {
				removeEndSeq = key.End
			}

			elem := list._remove(SkippedSequenceEntry{Start: removeStartSeq, End: removeEndSeq})
			if elem != nil {
				removedSeqs = true
				list.NumSequencesInList -= int64((removeEndSeq - removeStartSeq) + 1)
			}
			key.Start = removeEndSeq + 1
			if key.Start > key.End {
				break
			}
		}
		if removedSeqs {
			// return non nil as we have removed seqs
			return &Element{}
		}
	}

	return nil
}

// _remove is private function that removes a range from the list. This differs from Remove as it doesn't acquire mutex
// and doesn't handle removing a range over multiple entries. Removal from multiple entries is not needed for this function
// given its only called in compaction and when iterating through list to remove a range that spans multiple entries
func (list *SkipList) _remove(key SkippedSequenceEntry) *Element {
	prevs := list.getPrevElementNodes(key)

	// found the element, remove it
	if element := prevs[0].next[0]; element != nil && element.IsWithinRange(key) {
		if !element.IsRange() || (element.key.Start == key.Start && element.key.End == key.End) {
			for k, v := range element.next {
				prevs[k].next[k] = v
			}
			list.Length--
			return element
		}
		if key.Start == element.key.Start {
			element.key = SkippedSequenceEntry{Start: key.End + 1, End: element.key.End, Timestamp: element.key.Timestamp}
			// return non nil elem as we have found item
			return &Element{}
		}
		if key.End == element.key.End {
			element.key = SkippedSequenceEntry{Start: element.key.Start, End: key.Start - 1, Timestamp: element.key.Timestamp}
			// return non nil elem as we have found item
			return &Element{}
		}
		newEntryKey := SkippedSequenceEntry{Start: key.End + 1, End: element.key.End, Timestamp: element.key.Timestamp}
		newCurrElemKey := SkippedSequenceEntry{Start: element.key.Start, End: key.Start - 1, Timestamp: element.key.Timestamp}
		element.key = newCurrElemKey
		// insert new elem at
		list._set(newEntryKey)
		return &Element{}
	}

	return nil
}

// getPrevElementNodes is the private search mechanism that other functions use.
// Finds the previous nodes on each level relative to the current Element and
// caches them. This approach is similar to a "search finger" as described by Pugh:
// http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.17.524
func (list *SkipList) getPrevElementNodes(key SkippedSequenceEntry) []*elementNode {
	var prev *elementNode = &list.elementNode
	var next *Element

	prevs := list.prevNodesCache

	for i := list.maxLevel - 1; i >= 0; i-- {
		next = prev.next[i]

		for next != nil && key.Start > next.key.End {
			prev = &next.elementNode
			next = next.next[i]
		}

		prevs[i] = prev
	}

	return prevs
}

// SetProbability changes the current P value of the list.
// It doesn't alter any existing data, only changes how future insert heights are calculated.
func (list *SkipList) SetProbability(newProbability float64) {
	list.probability = newProbability
	list.probTable = probabilityTable(list.probability, list.maxLevel)
}

func (list *SkipList) randLevel() (level int) {
	// Our random number source only has Int63(), so we have to produce a float64 from it
	// Reference: https://golang.org/src/math/rand/rand.go#L150
	r := float64(list.randSource.Int63()) / (1 << 63)

	level = 1
	for level < list.maxLevel && r < list.probTable[level] {
		level++
	}
	return
}

// probabilityTable calculates in advance the probability of a new node having a given level.
// probability is in [0, 1], MaxLevel is (0, 64]
// Returns a table of floating point probabilities that each level should be included during an insert.
func probabilityTable(probability float64, MaxLevel int) (table []float64) {
	for i := 1; i <= MaxLevel; i++ {
		prob := math.Pow(probability, float64(i-1))
		table = append(table, prob)
	}
	return table
}

// NewWithMaxLevel creates a new skip list with MaxLevel set to the provided number.
// maxLevel has to be int(math.Ceil(math.Log(N))) for DefaultProbability (where N is an upper bound on the
// number of elements in a skip list). See http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.17.524
// Returns a pointer to the new list.
func NewWithMaxLevel(maxLevel int) *SkipList {
	if maxLevel < 1 || maxLevel > 64 {
		panic("maxLevel for a SkipList must be a positive integer <= 64")
	}

	return &SkipList{
		elementNode:    elementNode{next: make([]*Element, maxLevel)},
		prevNodesCache: make([]*elementNode, maxLevel),
		maxLevel:       maxLevel,
		randSource:     rand.New(rand.NewSource(time.Now().UnixNano())),
		probability:    DefaultProbability,
		probTable:      probabilityTable(DefaultProbability, maxLevel),
	}
}

// New creates a new skip list with default parameters. Returns a pointer to the new list.
func New() *SkipList {
	return NewWithMaxLevel(DefaultMaxLevel)
}

// Compacts the skiplist by removing elements that are older than maxWait.
func (list *SkipList) CompactList(timeNow, maxWait int64) int64 {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	numCompacted := int64(0)
	for c := list._front(); c != nil; c = c.Next() {
		if (timeNow - c.key.Timestamp) >= maxWait {
			prevs := list.getPrevElementNodes(c.Key())
			numCompacted += c.key.GetNumSequencesInEntry()
			for k, v := range c.next {
				prevs[k].next[k] = v
			}
			// remove element
			list.Length--
		}
	}
	list.NumSequencesInList -= numCompacted
	return numCompacted
}
