// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package persist

import (
	"fmt"
	"iter"
	"math/rand"
	"slices"
	"strings"
	"testing"
)

func TestAppendExhaustive(t *testing.T) {
	// add appends to tr n values with keys [val, val+n).
	add := func(tr *TransientSlice[int], n int, val int) {
		var list []int
		for i := range n {
			list = append(list, val+i)
		}
		tr.Append(list...)
	}

	// check checks that seq has i+j+k values
	// with indexes [0, i+j+k) and values [0, i), [100, 100+j), [200, 200+k).
	check := func(seq iter.Seq2[int, int], i, j, k int) {
		var have [][2]int
		var want [][2]int
		for k, v := range seq {
			have = append(have, [2]int{k, v})
		}
		for c := range i {
			want = append(want, [2]int{c, c})
		}
		for c := range j {
			want = append(want, [2]int{i + c, c + 100})
		}
		for c := range k {
			want = append(want, [2]int{i + j + c, c + 200})
		}
		if !slices.Equal(have, want) {
			t.Errorf("%d %d %d:\nhave=%v\nwant=%v", i, j, k, have, want)
		}
	}

	// Try all possible triples of appends.
	// Since chunk is 32, this covers all possible fragment alignments
	// that arise during the appends, as well as the growth of a new
	// level of the tree.
	var s0 Slice[int]
	for i := range 34 {
		for j := range 66 {
			for k := range 34 {
				tr := s0.Transient()
				add(tr, i, 0)
				add(tr, j, 100)
				add(tr, k, 200)
				check(tr.All(), i, j, k)
				check(tr.Persist().All(), i, j, k)
			}
		}
	}
}

func TestSmall(t *testing.T) {
	// Test small trees of every size, to catch boundary conditions.
	for i := range 100 {
		testN(t, i)
	}
}

func TestLarge(t *testing.T) {
	// Test a large tree > 32**3, so at least four levels.
	testN(t, 50001)
}

func testN(t *testing.T, N int) {
	// Build tree using Append.
	const V = 100000
	var s0 Slice[int]
	tr1 := s0.Transient()
	for i := range N {
		tr1.Append(V + i)
	}
	if n := tr1.Len(); n != N {
		t.Fatalf("tr1.Len() = %d, want %d", n, N)
	}
	// Reread it.
	for i := range N {
		j := tr1.At(i)
		if j != V+i {
			t.Fatalf("tr1.At(%d) = %d, want %d", i, j, V+i)
		}
	}

	// Check iterating and stopping.
	if N < 100 {
		testSlice(t, tr1, 1, V, true)
	}

	// Check bad indexes.
	wantPanic(t, fmt.Sprintf("index %d out of range [0:%d]", N, N), func() { tr1.At(N) })
	wantPanic(t, fmt.Sprintf("index %d out of range [0:%d]", N+1, N), func() { tr1.At(N + 1) })
	wantPanic(t, fmt.Sprintf("index %d out of range [0:%d]", -1, N), func() { tr1.At(-1) })

	// Check bad slices.
	wantPanic(t, fmt.Sprintf("slice [%d:%d] out of range [0:%d]", -1, N, N), func() { tr1.Slice(-1, N) })
	wantPanic(t, fmt.Sprintf("slice [%d:%d] out of range [0:%d]", 1, N+1, N), func() { tr1.Slice(1, N+1) })
	wantPanic(t, fmt.Sprintf("slice [%d:%d] out of range [0:%d]", 2, 1, N), func() { tr1.Slice(2, 1) })

	// Check bad set indexes.
	wantPanic(t, fmt.Sprintf("index %d out of range [0:%d]", N, N), func() { tr1.Set(N, 0) })

	// Check that Persist also works.
	s1 := tr1.Persist()
	for i := range N {
		j := s1.At(i)
		if j != 100000+i {
			t.Fatalf("s1.At(%d) = %d, want %d", i, j, 100000+i)
		}
	}

	wantPanic(t, fmt.Sprintf("index %d out of range [0:%d]", N, N), func() { tr1.At(N) })
	wantPanic(t, fmt.Sprintf("index %d out of range [0:%d]", N+1, N), func() { tr1.At(N + 1) })
	wantPanic(t, fmt.Sprintf("index %d out of range [0:%d]", -1, N), func() { tr1.At(-1) })

	// Overwrite tree using Set, in random order.
	// Check that it has the values we want,
	// and that tr1 and s1 are unchanged.
	tr2 := s1.Transient()
	for _, i := range rand.Perm(N) {
		tr2.Set(i, 200000+i)
		if j := tr2.At(i); j != 200000+i {
			t.Fatalf("tr2.At(%d) = %d, want %d", i, j, 200000+i)
		}
		if j := s1.At(i); j != 100000+i {
			t.Fatalf("after tr2.Set, s1.At(%d) = %d, want %d", i, j, 100000+i)
		}
		if j := tr1.At(i); j != 100000+i {
			t.Fatalf("after tr2.Set, tr1.At(%d) = %d, want %d", i, j, 100000+i)
		}
	}
}

func TestResize(t *testing.T) {
	var s0 Slice[int]
	tr := s0.Transient()

	const N = 1000
	for i := range N {
		tr.Append(i)
	}

	resizes := []int{
		N - 1,
		N &^ chunkMask,
		N/2 + 1,
		N/4 + 1,
		N / 4 &^ chunkMask,
		N - 1,
		N,
		2 * chunk,
		0,
		N,
		100,
		2 * chunk,
		0,
		100,
	}

	undefinedAt := N
	for _, size := range resizes {
		t.Logf("Resize(%d)", size)
		tr.Resize(size)
		undefinedAt = min(undefinedAt, size)
		if n := tr.Len(); n != size {
			t.Fatalf("tr.Len()=%d, want %d", n, size)
		}
		if h := height(tr.s.tlen); tr.s.height != h {
			t.Fatalf("tr.tlen=%d, tr.height=%d, want %d", tr.s.tlen, tr.s.height, h)
		}
		want := 0
		for i, v := range tr.All() {
			if want >= size {
				t.Fatalf("tr.All() returned %d,%d, want end of iteration", i, v)
			}
			if i != want || want < undefinedAt && v != want {
				wantV := fmt.Sprint(want)
				if want >= undefinedAt {
					wantV = "*"
				}
				t.Fatalf("tr.All() returned %d,%d, want %d,%s", i, v, want, wantV)
			}
			want = i + 1
		}
		if want != size {
			t.Fatalf("tr.All() stopped before %d, want before %d", want, size)
		}
	}
}

func TestHoles(t *testing.T) {
	// Test tree with holes from Resize beyond current size.
	const N = 100
	var s0 Slice[int]
	tr := s0.Transient()
	tr.Resize(N)
	if v := tr.At(N / 2); v != 0 {
		t.Fatalf("tr.At(%d) = %d, want %d", N/2, v, 0)
	}
	if v := tr.Persist().At(N / 2); v != 0 {
		t.Fatalf("tr.Persist().At(%d) = %d, want %d", N/2, v, 0)
	}
	testSlice(t, tr, 0, 0, true)
}

// wantPanic runs f and checks that it panics
// with a value whose string form contains text.
func wantPanic(t *testing.T, text string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		e := recover()
		if e == nil {
			t.Fatalf("no panic, wanted %q", text)
		}
		s := fmt.Sprint(e)
		if !strings.Contains(s, text) {
			t.Fatalf("panic(%q), wanted %q", s, text)
		}
	}()

	f()
}

// testSlice exercises all possible calls tr.Slice(i, j),
// expecting the value at position i to be i*m+a.
// If testBreak is true, testSlice tries each tr.Slice(i,j) call
// j-i+1 times, breaking the loop at every possible
// iteration count.
func testSlice(t *testing.T, tr *TransientSlice[int], m, a int, testBreak bool) {
	t.Helper()
	N := tr.Len()
	for i := range N {
		for j := i + 1; j <= N; j++ {
			k := j
			if testBreak {
				k = i
			}
			for ; k <= j; k++ {
				want := i
				for index, value := range tr.Slice(i, j) {
					if index != want || index > k || index >= j || value != index*m+a {
						t.Fatalf("tr.Slice(%d,%d): range produced %d,%d, want %d,%d", i, j, index, value, want, index*m+a)
					}
					if index == k {
						break
					}
					want = index + 1
				}
				if want != k {
					if k == j {
						t.Fatalf("tr.Slice(%d,%d): range did not stop just before %d", i, j, j)
					} else {
						t.Fatalf("tr.Slice(%d,%d): range did not reach %d", i, j, k)
					}
				}
			}
		}
	}
}

type slicer interface {
	Slice(i, j int) iter.Seq2[int, int]
	All() iter.Seq2[int, int]
}

func BenchmarkAppend(b *testing.B) {
	b.ReportAllocs()
	const N = 100000
	for b.Loop() {
		s := new(Slice[int])
		for i := range N {
			t := s.Transient()
			t.Append(i)
			s = t.Persist()
		}
	}
}
