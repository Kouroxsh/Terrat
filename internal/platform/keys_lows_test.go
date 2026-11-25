//go:build linux

package platform

import (
	"testing"

	"github.com/jezek/xgb/xproto"
)

func keyHandlerWith(syms ...xproto.Keysym) *KeyHandler {
	return &KeyHandler{
		minKeycode:        10,
		maxKeycode:        10,
		keysymsPerKeycode: byte(len(syms)),
		keysyms:           syms,
	}
}

func TestCtrlSpaceAndCtrlSlash(t *testing.T) {
	kh := keyHandlerWith(0x20)
	ev := xproto.KeyPressEvent{Detail: 10, State: ModCtrl}
	b, act := kh.Translate(ev)
	if act != ActionNone || len(b) != 1 || b[0] != 0x00 {
		t.Fatalf("Ctrl+Space = %q, act %v, want single 0x00 byte", b, act)
	}

	kh = keyHandlerWith(0x2f)
	b, act = kh.Translate(ev)
	if act != ActionNone || len(b) != 1 || b[0] != 0x1f {
		t.Fatalf("Ctrl+/ = %q, act %v, want single 0x1f byte", b, act)
	}
}

func TestCapsLockNonASCII(t *testing.T) {
	cases := []struct {
		syms  []xproto.Keysym
		state uint16
		want  xproto.Keysym
	}{
		{[]xproto.Keysym{0x07e1, 0x07c1}, ModLock, 0x07c1},
		{[]xproto.Keysym{0x06c1}, ModLock, 0x06e1},
		{[]xproto.Keysym{0x00e9}, ModLock, 0x00c9},
		{[]xproto.Keysym{0x03bb}, ModLock, 0x039b},
		{[]xproto.Keysym{0x61, 0x41}, ModLock | ModShift, 'a'},
		{[]xproto.Keysym{0x61, 0x41}, ModLock, 'A'},
	}
	for _, c := range cases {
		kh := keyHandlerWith(c.syms...)
		got := kh.KeySym(xproto.KeyPressEvent{Detail: 10, State: c.state})
		if got != c.want {
			t.Errorf("KeySym(%#x, state %#x) = %#x, want %#x", c.syms[0], c.state, got, c.want)
		}
	}
}

func TestFuncKeySeq(t *testing.T) {
	cases := []struct {
		sym              xproto.Keysym
		shift, alt, ctrl bool
		want             string
	}{
		{0xffbe, false, false, false, "\x1bOP"},
		{0xffbe, true, false, false, "\x1b[1;2P"},
		{0xffbe, false, false, true, "\x1b[1;5P"},
		{0xffbe, false, true, false, "\x1b[1;3P"},
		{0xffbe, true, true, true, "\x1b[1;8P"},
		{0xffc2, false, false, false, "\x1b[15~"},
		{0xffc2, true, false, false, "\x1b[15;2~"},
		{0xffc2, false, false, true, "\x1b[15;5~"},
		{0xffc9, false, false, false, "\x1b[24~"},
		{0xffca, false, false, false, "\x1b[25~"},
		{0xffca, false, false, true, "\x1b[25;5~"},
		{0xffd1, false, false, false, "\x1b[34~"},
	}
	for _, c := range cases {
		if got := funcKeySeq(c.sym, c.shift, c.alt, c.ctrl); string(got) != c.want {
			t.Errorf("funcKeySeq(%#x, %v, %v, %v) = %q, want %q", c.sym, c.shift, c.alt, c.ctrl, got, c.want)
		}
	}
	if got := funcKeySeq(0xffd2, false, false, false); got != nil {
		t.Fatalf("F21 must stay unmapped (no standard xterm code), got %q", got)
	}

	kh := keyHandlerWith(0xffbe)
	b, _ := kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModAlt})
	if string(b) != "\x1b[1;3P" {
		t.Fatalf("Alt+F1 = %q, want %q", b, "\x1b[1;3P")
	}

	kh = keyHandlerWith(0xffca)
	b, _ = kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModShift})
	if string(b) != "\x1b[25;2~" {
		t.Fatalf("Shift+F13 = %q, want %q", b, "\x1b[25;2~")
	}
}

func TestShiftNavKeysAltScreen(t *testing.T) {
	kh := keyHandlerWith(0xff55)
	ev := xproto.KeyPressEvent{Detail: 10, State: ModShift}
	if _, act := kh.Translate(ev); act != ActionScrollUp {
		t.Fatalf("Shift+PgUp (no flags) act = %v, want ActionScrollUp", act)
	}
	if _, act := kh.Translate(ev, false, false); act != ActionScrollUp {
		t.Fatalf("Shift+PgUp (normal screen) act = %v, want ActionScrollUp", act)
	}
	b, act := kh.Translate(ev, false, true)
	if act != ActionNone || string(b) != "\x1b[5;2~" {
		t.Fatalf("Shift+PgUp (alt screen) = %q, act %v, want %q", b, act, "\x1b[5;2~")
	}

	kh = keyHandlerWith(0xff56)
	b, act = kh.Translate(ev, false, true)
	if act != ActionNone || string(b) != "\x1b[6;2~" {
		t.Fatalf("Shift+PgDn (alt screen) = %q, act %v, want %q", b, act, "\x1b[6;2~")
	}
	kh = keyHandlerWith(0xff50)
	b, act = kh.Translate(ev, false, true)
	if act != ActionNone || string(b) != "\x1b[1;2H" {
		t.Fatalf("Shift+Home (alt screen) = %q, act %v, want %q", b, act, "\x1b[1;2H")
	}
	kh = keyHandlerWith(0xff57)
	b, act = kh.Translate(ev, false, true)
	if act != ActionNone || string(b) != "\x1b[1;2F" {
		t.Fatalf("Shift+End (alt screen) = %q, act %v, want %q", b, act, "\x1b[1;2F")
	}

	kh = keyHandlerWith(0xff9a)
	b, act = kh.Translate(ev, false, true)
	if act != ActionNone || string(b) != "\x1b[5;2~" {
		t.Fatalf("Shift+KP_Prior (alt screen) = %q, act %v, want %q", b, act, "\x1b[5;2~")
	}
}

func TestKeySymShortMappingNoPanic(t *testing.T) {
	kh := &KeyHandler{
		minKeycode:        10,
		maxKeycode:        10,
		keysymsPerKeycode: 7,
		keysyms:           []xproto.Keysym{0x61},
	}

	ev := xproto.KeyPressEvent{Detail: 10, State: 3 << 13}
	if got := kh.KeySym(ev); got != 0x61 {
		t.Fatalf("KeySym with short mapping = %#x, want %#x (group fallback)", got, 0x61)
	}

	ev = xproto.KeyPressEvent{Detail: 10, State: ModShift}
	if got := kh.KeySym(ev); got != 0x61 {
		t.Fatalf("KeySym with short mapping + Shift = %#x, want %#x", got, 0x61)
	}

	kh = &KeyHandler{minKeycode: 10, maxKeycode: 10, keysymsPerKeycode: 1, keysyms: nil}
	if got := kh.KeySym(xproto.KeyPressEvent{Detail: 10}); got != 0 {
		t.Fatalf("KeySym with empty mapping = %#x, want 0", got)
	}

	kh = &KeyHandler{minKeycode: 10, maxKeycode: 12, keysymsPerKeycode: 2, keysyms: []xproto.Keysym{0x61, 0x41}}
	if got := kh.KeySym(xproto.KeyPressEvent{Detail: 11}); got != 0 {
		t.Fatalf("KeySym for unmapped keycode = %#x, want 0", got)
	}
}
