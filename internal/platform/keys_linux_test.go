//go:build linux

package platform

import (
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

func TestKeysymToRune(t *testing.T) {
	// ASCII
	r, ok := KeysymToRune('a')
	if !ok || r != 'a' {
		t.Fatalf("expected 'a', got %q, %v", r, ok)
	}

	// Latin-1
	r, ok = KeysymToRune(0x00e9) // é
	if !ok || r != 'é' {
		t.Fatalf("expected 'é', got %q, %v", r, ok)
	}

	// Direct Unicode and flagged Unicode keysyms.
	for _, sym := range []xproto.Keysym{0x03bb, 0x010003bb} {
		r, ok := KeysymToRune(sym)
		if !ok || r != 'λ' {
			t.Fatalf("expected lambda, got %q, %v", r, ok)
		}
	}

	// Numpad / Keypad digits 0-9
	for i := 0; i <= 9; i++ {
		sym := 0xffb0 + i
		r, ok := KeysymToRune(xproto.Keysym(sym))
		expected := rune('0' + i)
		if !ok || r != expected {
			t.Fatalf("expected KP_%d to be %q, got %q", i, expected, r)
		}
	}

	// Numpad operators
	kpOps := map[xproto.Keysym]rune{
		0xffaa: '*',
		0xffab: '+',
		0xffac: ',',
		0xffad: '-',
		0xffae: '.',
		0xffaf: '/',
		0xffbd: '=',
		0xff80: ' ',
		0xff89: '\t',
		0xff8d: '\r',
	}
	for sym, expected := range kpOps {
		r, ok := KeysymToRune(sym)
		if !ok || r != expected {
			t.Fatalf("expected keysym 0x%x to be %q, got %q", sym, expected, r)
		}
	}
}

func TestTranslateNumpadAndAppCursor(t *testing.T) {
	kh := &KeyHandler{}

	// Test Numpad digits translate
	for i := 0; i <= 9; i++ {
		sym := uint32(0xffb0 + i)
		ev := xproto.KeyPressEvent{}
		kh.keysyms = []xproto.Keysym{xproto.Keysym(sym)}
		kh.keysymsPerKeycode = 1
		kh.minKeycode = 10
		kh.maxKeycode = 10
		ev.Detail = 10

		b, act := kh.Translate(ev)
		if act != ActionNone || len(b) != 1 || b[0] != byte('0'+i) {
			t.Fatalf("expected numpad digit %d, got b=%q, act=%v", i, string(b), act)
		}
	}

	// Test AppCursor Mode for arrow keys
	kh.keysyms = []xproto.Keysym{0xff52} // Up Arrow
	ev := xproto.KeyPressEvent{Detail: 10}

	// Normal mode (appCursor = false)
	b, _ := kh.Translate(ev, false)
	if string(b) != "\x1b[A" {
		t.Fatalf("expected \\x1b[A in normal cursor mode, got %q", string(b))
	}

	// AppCursor mode (appCursor = true)
	b, _ = kh.Translate(ev, true)
	if string(b) != "\x1bOA" {
		t.Fatalf("expected \\x1bOA in appCursor mode, got %q", string(b))
	}
}

func TestXKBState(t *testing.T) {
	conn, err := xgb.NewConn()
	if err != nil {
		t.Skip("no X server")
	}
	defer conn.Close()

	ext, err := xproto.QueryExtension(conn, uint16(len("XKEYBOARD")), "XKEYBOARD").Reply()
	if err != nil || ext == nil || !ext.Present {
		t.Skip("XKEYBOARD not present")
	}

	buf := make([]byte, 8)
	buf[0] = ext.MajorOpcode
	buf[1] = 0 // X_kbUseExtension
	xgb.Put16(buf[2:], 2)
	xgb.Put16(buf[4:], 1)
	xgb.Put16(buf[6:], 0)

	cookie := conn.NewCookie(true, true)
	conn.NewRequest(buf, cookie)
	reply, err := cookie.Reply()
	if err != nil {
		t.Fatalf("X_kbUseExtension error: %v", err)
	}
	t.Logf("X_kbUseExtension supported=%v", reply[1] != 0)

	reqState := make([]byte, 8)
	reqState[0] = ext.MajorOpcode
	reqState[1] = 4 // X_kbGetState
	xgb.Put16(reqState[2:], 2)
	xgb.Put16(reqState[4:], 0x0100) // XkbUseCoreKbd

	cookieState := conn.NewCookie(true, true)
	conn.NewRequest(reqState, cookieState)
	replyState, err := cookieState.Reply()
	if err != nil {
		t.Fatalf("X_kbGetState error: %v", err)
	}
	t.Logf("X_kbGetState group=%d, lockedGroup=%d", replyState[12], replyState[13])
	reqSelect := make([]byte, 16)
	reqSelect[0] = ext.MajorOpcode
	reqSelect[1] = 1                  // X_kbSelectEvents
	xgb.Put16(reqSelect[2:], 4)       // 16 bytes / 4
	xgb.Put16(reqSelect[4:], 0x0100)  // XkbUseCoreKbd
	xgb.Put16(reqSelect[6:], 0x0010)  // affectWhich: XkbStateNotify (1<<4)
	xgb.Put16(reqSelect[8:], 0)       // clear
	xgb.Put16(reqSelect[10:], 0x0010) // selectAll: XkbStateNotify (1<<4)
	xgb.Put16(reqSelect[12:], 0)      // affectMap
	xgb.Put16(reqSelect[14:], 0)      // map

	cookieSelect := conn.NewCookie(true, false)
	conn.NewRequest(reqSelect, cookieSelect)
	if err := cookieSelect.Check(); err != nil {
		t.Fatalf("X_kbSelectEvents error: %v", err)
	}
	t.Logf("X_kbSelectEvents succeeded!")
}

func TestUnicodeGroupTranslation(t *testing.T) {
	kh := &KeyHandler{
		minKeycode:        38,
		maxKeycode:        38,
		keysymsPerKeycode: 4,
		keysyms: []xproto.Keysym{
			'a', 'A', 0x010003bb, 0x0100039b, // lambda, capital lambda
		},
	}

	// 1. Group 0 (English)
	evEN := xproto.KeyPressEvent{Detail: 38, State: 0}
	bEN, _ := kh.Translate(evEN)
	if string(bEN) != "a" {
		t.Fatalf("expected 'a' in group 0, got %q", string(bEN))
	}

	// 2. Group 1 via ev.State (bits 13-14)
	evGroup := xproto.KeyPressEvent{Detail: 38, State: 1 << 13}
	bGroup, _ := kh.Translate(evGroup)
	if string(bGroup) != "λ" {
		t.Fatalf("expected 'λ' in group 1 via State, got %q", string(bGroup))
	}

	// 3. Group 1 via kh.activeGroup
	kh.SetGroup(1)
	evGroup2 := xproto.KeyPressEvent{Detail: 38, State: 0}
	bGroup2, _ := kh.Translate(evGroup2)
	if string(bGroup2) != "λ" {
		t.Fatalf("expected 'λ' in group 1 via activeGroup, got %q", string(bGroup2))
	}
}
