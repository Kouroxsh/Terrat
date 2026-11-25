package terminal

import "testing"

func TestWriteUTF8SplitAtEveryByte(t *testing.T) {
	text := []byte("café Ω 世界 😀")
	for split := 0; split <= len(text); split++ {
		term := New(80, 2)
		n, err := term.Write(text[:split])
		if err != nil || n != split {
			t.Fatalf("first write: %d, %v", n, err)
		}
		n, err = term.Write(text[split:])
		if err != nil || n != len(text)-split {
			t.Fatalf("second write: %d, %v", n, err)
		}
		want := New(80, 2)
		want.Write(text)
		for x := 0; x < 80; x++ {
			if got := term.GetCell(x, 0); !got.Equal(want.GetCell(x, 0)) {
				t.Fatalf("split %d column %d: got %+v, want %+v", split, x, got, want.GetCell(x, 0))
			}
		}
	}
	term := New(20, 2)
	for _, b := range []byte("世界") {
		term.Write([]byte{b})
	}

	if got := term.GetCell(0, 0); got.Char != '世' || !got.Wide {
		t.Fatalf("column 0: got %+v, want lead cell of U+4E16", got)
	}
	if got := term.GetCell(1, 0); !got.Continuation {
		t.Fatalf("column 1: got %+v, want continuation cell", got)
	}
	if got := term.GetCell(2, 0); got.Char != '界' || !got.Wide {
		t.Fatalf("column 2: got %+v, want lead cell of U+754C", got)
	}
	if got := term.GetCell(3, 0); !got.Continuation {
		t.Fatalf("column 3: got %+v, want continuation cell", got)
	}
}

func TestWriteInvalidUTF8Recovers(t *testing.T) {
	term := New(20, 2)
	term.Write([]byte{0xd8})
	term.Write([]byte{'X', 0xff, 'Y'})
	for i, r := range []rune("�X�Y") {
		if got := term.GetCell(i, 0).Char; got != r {
			t.Fatalf("column %d: got %U, want %U", i, got, r)
		}
	}
}
