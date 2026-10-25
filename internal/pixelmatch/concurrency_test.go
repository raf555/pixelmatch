package pixelmatch

import (
	"bytes"
	"sync"
	"testing"

	"github.com/raf555/pixelmatch/v7/internal/testutil"
)

// TestMatchConcurrent runs the reference cases from several goroutines at
// once. Results must still match upstream byte for byte. Run with -race to
// check for shared state.
func TestMatchConcurrent(t *testing.T) {
	cases, err := testutil.GetTestCases()
	if err != nil {
		t.Fatal(err)
	}

	const workers = 8
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for _, c := range cases {
				opts := refOptions(c)
				out := make([]byte, len(c.Img1))
				n, err := Match(c.Img1, c.Img2, out, c.W, c.H, &opts)
				if err != nil {
					t.Errorf("%s: %v", c.Name, err)
					return
				}
				if n != c.N {
					t.Errorf("%s: n = %d, want %d", c.Name, n, c.N)
				}
				if !bytes.Equal(out, c.Diff) {
					t.Errorf("%s: diff output differs from upstream", c.Name)
				}
			}
		})
	}
	wg.Wait()
}
