package diff

import (
	"fmt"
)

// TODO: coalesce `-` followed by `+` into a change (if it looks
// similar?)
func DiffLines(a, b []string, path string) ([]Difference, error) {
	if len(a) == 0 && len(b) == 0 {
		return nil, nil
	}

	var diffs []Difference
	addIndex, removeIndex := 0, 0

	chunks := diffChunks(a, b)
	for _, chunk := range chunks {
		for i, line := range chunk.Deleted {
			diffs = append(diffs, Removed{line, fmt.Sprintf("%s[%d]", path, removeIndex+i)})
		}
		removeIndex += len(chunk.Deleted) + len(chunk.Equal)

		for i, line := range chunk.Added {
			diffs = append(diffs, Added{line, fmt.Sprintf("%s[%d]", path, addIndex+i)})
		}
		addIndex += len(chunk.Added) + len(chunk.Equal)
	}
	return diffs, nil
}

// Taken from https://gowalker.org/github.com/kylelemons/godebug/diff
func diffChunks(A, B []string) []Chunk {
	// algorithm: http://www.xmailserver.org/diff2.pdf
	N, M := len(A), len(B)
	MAX := N + M
	V := make([]int, 2*MAX+1)
	Vs := make([][]int, 0, 8)

	var D int
dLoop:
	for D = 0; D <= MAX; D++ {
		for k := -D; k <= D; k += 2 {
			var x int
			if k == -D || (k != D && V[MAX+k-1] < V[MAX+k+1]) {
				x = V[MAX+k+1]
			} else {
				x = V[MAX+k-1] + 1
			}
			y := x - k
			for x < N && y < M && A[x] == B[y] {
				x++
				y++
			}
			V[MAX+k] = x
			if x >= N && y >= M {
				Vs = append(Vs, append(make([]int, 0, len(V)), V...))
				break dLoop
			}
		}
		Vs = append(Vs, append(make([]int, 0, len(V)), V...))
	}
	if D == 0 {
		return nil
	}
	chunks := make([]Chunk, D+1)

	x, y := N, M
	for d := D; d > 0; d-- {
		V := Vs[d]
		k := x - y
		insert := k == -d || (k != d && V[MAX+k-1] < V[MAX+k+1])

		x1 := V[MAX+k]
		var x0, xM, kk int
		if insert {
			kk = k + 1
			x0 = V[MAX+kk]
			xM = x0
		} else {
			kk = k - 1
			x0 = V[MAX+kk]
			xM = x0 + 1
		}
		y0 := x0 - kk

		var c Chunk
		if insert {
			c.Added = B[y0:][:1]
		} else {
			c.Deleted = A[x0:][:1]
		}
		if xM < x1 {
			c.Equal = A[xM:][:x1-xM]
		}

		x, y = x0, y0
		chunks[d] = c
	}
	if x > 0 {
		chunks[0].Equal = A[:x]
	}
	return chunks
}

type Chunk struct {
	Added   []string
	Deleted []string
	Equal   []string
}
