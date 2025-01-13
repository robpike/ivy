// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package persist implements a persistent slice data structure,
// similar to [Clojure's persistent vectors].
// The meaning of “persistent” here is that operations on the slice
// create a new slice, with the old versions continuing to be valid.
// The [Slice] type is the persistent slice, and a batch of changes
// can be made to it using the [TransientSlice] type.
//
// [Clojure's persistent vectors]: https://hypirion.com/musings/understanding-persistent-vector-pt-1
package persist

import (
	"fmt"
	"iter"
	"math/bits"
	"slices"
	"sync"
	"sync/atomic"
)

// A Slice is an immutable slice of values.
// It can be read but not written.
// To create a modified version of the slice,
// call [Slice.Transient] to obtain a [TransientSlice],
// make the changes in the transient,
// and then call [Transient.Persist] to obtain a new [Slice].
type Slice[T any] struct {
	// A Slice is a 16-way indexed tree (technically a trie) of values. That
	// is, each leaf node in the tree holds 16 values, each interior node
	// (inode) holds 16 pointers to the next level, and the tree is perfectly
	// balanced, since the keys are [0, tlen). For cheaper appends and small
	// slices, only full chunks are stored in the tree: the final fragment is
	// in tail. The tree can have “holes”, meaning a nil where a leaf or
	// inode pointer should be; holes are treated as full of zero values.
	//
	// The inode pointer values are atomic.Value because when the nodes are
	// part of a TransientSlice, nearby Set operations (or a Set and a nearby
	// At) may be racing to access the pointers at the same time. The tree
	// value is atomic.Value to match the inode pointers. Since atomic.Value
	// requires having only a single concrete type over its entire lifetime,
	// tree is always nil or a *inode (sometimes (*inode)(nil)). As a special
	// case, the tree of height 0 may have tree set to any(nil) or
	// (*inode)(nil), and there are no trees of height 1 (which would need a
	// root of type *leaf[T]).
	tree   atomic.Value // tree of chunks
	height int          // height of tree
	tlen   int          // number of elements in tree (excludes tail)
	tail   []T          // pending values (up to chunk) to flush to tree
}

// A TransientSlice is a mutable slice of values,
// typically derived from and intended to become a [Slice].
type TransientSlice[T any] struct {
	// A TransientSlice is the mutable form of a Slice. It uses the same
	// 16-way tree, but copy-on-write. More specifically, the TransientSlice
	// keeps track of which nodes and leaves it owns, meaning they are not
	// shared with any Slices. Those can be modified directly. Other nodes
	// and leaves must be copied before being modified.
	//
	// Each TransientSlice has a unique ID (the aid field below, accessed by
	// [TransientSlice.id]). A node or leaf with the same ID is owned by that
	// TransientSlice and writable; others are copy-on-write.
	// The [TransientSlice.Persist] operation picks a new top-level ID, giving up
	// ownership of all the previously owned nodes and leaves and making them
	// safe to publish as a Slice.
	//
	// In addition to the tree structure, a Slice has a tail of up to 16
	// elements not yet stored in the tree, cutting tree updates by 16X.
	// The wtail field tracks whether s.tail is owned by the TransientSlice,
	// meaning can be written to. If wtail is true, s.tail has capacity 16.
	//
	// The tailLen is a cached copy of len(s.tail), necessary because
	// [TransientSlice.writeTail] makes a copy of and reassigns s.tail.
	// This might happen during t.Set(i, ...) and does not change len(s.tail),
	// so conceptually t.Len() should be permitted concurrent with t.Set(i, ...),
	// but technically reading len(s.tail) during t.Len and writing it
	// during writeTail during t.Set is a race. Instead, we only set s.tail
	// using t.setTail, which caches len(s.tail) in t.tailLen.
	s       Slice[T]      // underlying slice data structure
	aid     atomic.Uint64 // id of this transient (see [TransientSlice.id])
	tailLen int           // cached copy of len(t.s.tail)
	wtail   atomic.Bool   // whether s.tail is writable
	tailMu  sync.Mutex    // protects tail copy in writeTail
}

// transientID is the most recently used transient ID.
// To obtain a new ID, use transientID.Add(1).
var transientID atomic.Uint64

// id returns the id for this TransientSlice, copied from t.aid.
// If an id has not been allocated yet (t.aid==0),
// id sets t.aid to a new id and returns that id.
func (t *TransientSlice[T]) id() uint64 {
	for {
		id := t.aid.Load()
		if id == 0 {
			id = transientID.Add(1)
			if !t.aid.CompareAndSwap(0, id) {
				// Racing with another t.id call.
				continue
			}
		}
		return id
	}
}

// The tree uses chunks of size 16 and 16-way branching in the interior nodes.
// A power of two is convenient.
// Clojure uses 32, but BenchmarkAppend runs fastest with 16.
const (
	chunkBits = 4
	chunkMask = chunk - 1
	chunk     = 1 << chunkBits
)

// height returns the height of a tree with tlen elements.
func height(tlen int) int {
	if tlen == 0 {
		return 0
	}
	return max(2, 1+bits.Len(uint(tlen-1))/chunkBits)
}

// A leaf is a leaf node in the tree.
type leaf[T any] struct {
	val [chunk]T // values
	id  uint64   // id of TransientSlice that can write this leaf
}

// An inode is an interior node in the tree.
type inode struct {
	ptr [chunk]atomic.Value // all *leaf[T] or all *node, depending on tree level
	id  uint64              // id of TransientSlice that can write this node
}

// Len returns len(s).
func (s *Slice[T]) Len() int {
	return s.tlen + len(s.tail)
}

// Len returns len(t).
func (t *TransientSlice[T]) Len() int {
	// Note: using t.tailLen to avoid race with t.writeTail overwriting t.s.tail
	// (but not changing len(t.s.tail)).
	return t.s.tlen + t.tailLen
}

// At returns s[i].
func (s *Slice[T]) At(i int) T {
	if i < 0 || i >= s.Len() {
		panic(fmt.Sprintf("index %d out of range [0:%d]", i, s.Len()))
	}
	if i >= s.tlen {
		return s.tail[i&chunkMask]
	}
	p := s.tree.Load()
	for shift := (s.height - 1) * chunkBits; shift > 0 && p != nil; shift -= chunkBits {
		p = p.(*inode).ptr[(i>>shift)&chunkMask].Load()
	}
	if p == nil {
		var zero T
		return zero
	}
	return p.(*leaf[T]).val[i&chunkMask]
}

// At returns t[i].
func (t *TransientSlice[T]) At(i int) T { return t.s.At(i) }

// All returns an iterator over s[0:len(s)].
func (s *Slice[T]) All() iter.Seq2[int, T] {
	return s.Slice(0, s.Len())
}

// All returns an iterator over t[0:len(t)].
func (t *TransientSlice[T]) All() iter.Seq2[int, T] { return t.s.All() }

// Slice returns an iterator over s[i:j].
func (s *Slice[T]) Slice(i, j int) iter.Seq2[int, T] {
	if i < 0 || j < i || j > s.Len() {
		panic(fmt.Sprintf("slice [%d:%d] out of range [0:%d]", i, j, s.Len()))
	}
	return func(yield func(int, T) bool) {
		if i < s.tlen && !s.yield(&s.tree, s.height-1, i, min(j, s.tlen), yield) {
			return
		}
		for k := max(i, s.tlen); k < j; k++ {
			if !yield(k, s.tail[k-s.tlen]) {
				return
			}
		}
	}
}

// Slice returns an iterator over t[i:j].
func (t *TransientSlice[T]) Slice(i, j int) iter.Seq2[int, T] { return t.s.Slice(i, j) }

// yield calls yield(i, s[i]) for each element in s[start:end],
// stopping and returning false if any of the yield calls return false.
// p is a node at the given level (level 0 is leaves) and covers all of s[start:end].
func (s *Slice[T]) yield(p *atomic.Value, level, start, end int, yield func(int, T) bool) bool {
	pl := p.Load()
	if pl == nil {
		var zero T
		for ; start < end; start++ {
			if !yield(start, zero) {
				return false
			}
		}
		return true
	}

	if level == 0 {
		l := pl.(*leaf[T])
		for i := range end - start {
			if !yield(start+i, l.val[start&chunkMask+i]) {
				return false
			}
		}
		return true
	}

	// Interior node.
	ip := pl.(*inode)

	shift := level * chunkBits
	width := 1 << shift // width of subtree of each child
	for j := (start >> shift) & chunkMask; j < chunk && start < end; j++ {
		m := min(end-start, width-start&(width-1))
		if !s.yield(&ip.ptr[j], level-1, start, start+m, yield) {
			return false
		}
		start += m
	}
	if start != end {
		// unreachable
		panic("persist: internal error: invalid yield")
	}
	return true
}

// Transient returns a TransientSlice for modifying (a copy of) s.
func (s *Slice[T]) Transient() *TransientSlice[T] {
	t := &TransientSlice[T]{s: *s}
	t.setTail(s.tail)
	return t
}

// Persist returns a [Slice] corresponding to the current state of t.
// Future modifications of t will not affect the returned slice.
func (t *TransientSlice[T]) Persist() *Slice[T] {
	s := t.s
	t.aid.Store(0)
	if t.wtail.Load() {
		s.tail = slices.Clone(s.tail)
	}
	return &s
}

// wleaf returns a writable version of the leaf *p.
// It implements the "create on write" or "copy on write"
// logic needed when *p is missing or shared with other Slice[T].
func (t *TransientSlice[T]) wleaf(p *atomic.Value) *leaf[T] {
	for {
		pl := p.Load()
		l, _ := pl.(*leaf[T]) // could be nil any
		tid := t.id()
		if l == nil || l.id != tid { // create or copy-on-write
			l1 := new(leaf[T])
			if l != nil {
				// It is safe to copy *l, because l is shared
				// and therefore no longer being modified.
				*l1 = *l
			}
			l1.id = tid
			if !p.CompareAndSwap(pl, l1) {
				// Racing with another t.wleaf (perhaps concurrent Set(i) and Set(i+1)).
				continue
			}
			l = l1
		}
		return l
	}
}

// wnode returns a writable version of the inode *p.
// It implements the "create on write" or "copy on write"
// logic needed when *p is missing or shared with other Slice[T].
func (t *TransientSlice[T]) wnode(p *atomic.Value) *inode {
	for {
		pl := p.Load()
		ip, _ := pl.(*inode) // could be nil any
		tid := t.id()
		if ip == nil || ip.id != tid { // create or copy-on-write
			ip1 := new(inode)
			if ip != nil {
				// *ip1 = *ip, but avoiding direct reads and writes
				// of the atomic.Value fields.
				for i := range ip1.ptr {
					if ptr := ip.ptr[i].Load(); ptr != nil {
						ip1.ptr[i].Store(ptr)
					}
				}
			}
			ip1.id = tid
			if !p.CompareAndSwap(pl, ip1) {
				// Racing with another t.wnode (perhaps concurrent Set(i) and Set(i+1)).
				continue
			}
			ip = ip1
		}
		return ip
	}
}

// growTree grows the tree t.s.tree to size tlen,
// adding new height levels as needed.
// The newly accessible content is undefined
// and must be initialized by the caller.
func (t *TransientSlice[T]) growTree(tlen int) {
	if tlen < t.s.tlen {
		// unreachable
		panic("persist: internal error: invalid growTree")
	}
	t.s.tlen = tlen
	h := height(tlen)
	if h == t.s.height {
		return
	}
	if t.s.height == 0 {
		// Nothing in tree yet; use nil root for new height.
		t.s.tree.Store((*inode)(nil))
		t.s.height = h
		return
	}
	root, _ := t.s.tree.Load().(*inode)
	tid := t.id()
	for ; t.s.height < h; t.s.height++ {
		ip := new(inode)
		ip.id = tid
		ip.ptr[0].Store(root)
		root = ip
	}
	t.s.tree.Store(root)
}

// shrinkTree shrinks the tree t.s.tree to size tlen,
// removing height levels as needed.
func (t *TransientSlice[T]) shrinkTree(tlen int) {
	t.s.tlen = tlen
	h := height(tlen)
	if h == t.s.height {
		return
	}
	if h == 0 {
		// Nothing in tree anymore; use nil root for empty tree.
		t.s.tree.Store((*inode)(nil))
		t.s.height = 0
		return
	}
	root, _ := t.s.tree.Load().(*inode)
	for ; t.s.height > h; t.s.height-- {
		if root != nil {
			root, _ = root.ptr[0].Load().(*inode)
		}
	}
	t.s.tree.Store(root)
}

// Set sets t[i] = x.
func (t *TransientSlice[T]) Set(i int, x T) {
	if i < 0 || i >= t.Len() {
		panic(fmt.Sprintf("index %d out of range [0:%d]", i, t.Len()))
	}

	// Write into tail?
	if i >= t.s.tlen {
		t.writeTail()
		t.s.tail[i&chunkMask] = x
		return
	}

	// Write into tree.
	p := &t.s.tree
	for b := (t.s.height - 1) * chunkBits; b > 0; b -= chunkBits {
		p = &t.wnode(p).ptr[(i>>b)&chunkMask]
	}
	t.wleaf(p).val[i&chunkMask] = x
}

// writeTail makes sure that t.s.tail is writable.
// Typically the caller has checked that it is not (t.rwtail is false),
// in which case writeTail replaces t.s.tail with a copy
// and sets t.rwtail to true.
// The copy has the same length but capacity set to chunk.
func (t *TransientSlice[T]) writeTail() {
	if t.wtail.Load() {
		return
	}
	t.tailMu.Lock()
	defer t.tailMu.Unlock()
	if t.wtail.Load() {
		return
	}

	tail := make([]T, len(t.s.tail), chunk)
	copy(tail, t.s.tail)
	t.setTail(tail)
	t.wtail.Store(true)
}

func (t *TransientSlice[T]) setTail(tail []T) {
	t.s.tail = tail
	if t.tailLen != len(tail) {
		t.tailLen = len(tail)
	}
}

// Append appends the src elements to t.
func (t *TransientSlice[T]) Append(src ...T) {
	if len(src) == 0 {
		return
	}

	// Append fragment to complete tail.
	if len(t.s.tail) > 0 {
		t.writeTail()
		n := copy(t.s.tail[len(t.s.tail):cap(t.s.tail)], src)
		t.setTail(t.s.tail[:len(t.s.tail)+n])
		if src = src[n:]; len(src) == 0 {
			return
		}

		// Tail is full with more to write; append tail to tree.
		t.appendTree(t.s.tail, chunk)
		t.setTail(t.s.tail[:0])
	}

	// Flush full chunks directly from src.
	if len(src) >= chunk {
		n := len(src) >> chunkBits << chunkBits
		t.appendTree(src, n)
		if src = src[n:]; len(src) == 0 {
			return
		}
	}

	// Copy fragment to tail.
	t.writeTail()
	t.setTail(append(t.s.tail, src...))
}

// appendTree appends xs (an integral number of chunks) to the tree.
func (t *TransientSlice[T]) appendTree(src []T, total int) {
	if total&chunkMask != 0 || total == 0 {
		// unreachable
		panic("persist: internal error: invalid appendTree")
	}

	// Update length, adding height to tree if needed.
	off := t.s.tlen
	t.growTree(off + total)

	// Copy new data into tree.
	t.copy(&t.s.tree, t.s.height-1, off, src, total)
}

// copy is like copy(t[off:], src[:total]),
// where p points to a node at the given level of the tree.
func (t *TransientSlice[T]) copy(p *atomic.Value, level, off int, src []T, total int) {
	if level == 0 {
		// Leaf level.
		l := t.wleaf(p)
		l.val = [chunk]T(src[:chunk])
		return
	}

	// Interior node.
	n := t.wnode(p)

	// Copy parts of xs into the appropriate child nodes.
	shift := level * chunkBits
	width := 1 << shift // width of subtree of each child
	for j := (off >> shift) & chunkMask; j < chunk && total > 0; j++ {
		m := min(total, width-off&(width-1))
		var next []T
		next, src = src[:m], src[m:]
		t.copy(&n.ptr[j], level-1, off, next, m)
		off += m
		total -= m
	}
	if total != 0 {
		// unreachable
		panic("persist: internal error: invalid copy")
	}
}

// Resize resizes t to have n elements.
// If t is being grown, the value of new elements is undefined.
func (t *TransientSlice[T]) Resize(n int) {
	tlen, tail := n&^chunkMask, n&chunkMask
	switch {
	case n > t.Len():
		// Grow.
		t.writeTail()
		if tlen != t.s.tlen {
			// Flush tail into tree and then grow tree more if needed.
			t.appendTree(t.s.tail[:chunk], chunk)
			t.growTree(tlen)
		}
		t.setTail(t.s.tail[:tail])
	case n < t.Len():
		// Shrink. May need to load different tail from tree before shrinking tree.
		if tlen != t.s.tlen {
			t.writeTail()
			p := &t.s.tree
			for b := (t.s.height - 1) * chunkBits; b > 0; b -= chunkBits {
				ip, _ := p.Load().(*inode)
				if ip == nil {
					p = nil
					break
				}
				p = &ip.ptr[(tlen>>b)&chunkMask]
			}
			var l *leaf[T]
			if p != nil {
				l, _ = p.Load().(*leaf[T])
			}
			if l == nil {
				clear(t.s.tail[:tail])
			} else {
				copy(t.s.tail[:tail], l.val[:tail])
			}
			t.shrinkTree(tlen)
		}
		t.setTail(t.s.tail[:tail])
	}
	if n != t.Len() {
		// unreachable
		panic("persist: internal error: invalid Resize")
	}
	return
}
