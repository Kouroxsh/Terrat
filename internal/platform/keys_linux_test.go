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

func testKeyHandler(syms []xproto.Keysym) *KeyHandler {
	return &KeyHandler{
		minKeycode:        10,
		maxKeycode:        10,
		keysymsPerKeycode: 1,
		keysyms:           syms,
	}
}

func TestTranslateShiftTabAndDeadKeys(t *testing.T) {
	// Shift+Tab: XKB reports ISO_Left_Tab (0xfe20), not 0xff09.
	kh := testKeyHandler([]xproto.Keysym{0xfe20})
	b, act := kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModShift})
	if act != ActionNone || string(b) != "\x1b[Z" {
		t.Fatalf("Shift+Tab: expected \\x1b[Z, got %q action %v", string(b), act)
	}

	// Ctrl+Shift+Tab is still "previous tab".
	_, act = kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModShift | ModCtrl})
	if act != ActionPrevTab {
		t.Fatalf("Ctrl+Shift+Tab: expected ActionPrevTab, got %v", act)
	}

	// Dead keys, AltGr and other ISO keysyms produce nothing.
	for _, sym := range []xproto.Keysym{0xfe50, 0xfe51, 0xfe03, 0xfe08, 0xff20, 0xfd00} {
		kh := testKeyHandler([]xproto.Keysym{sym})
		b, act := kh.Translate(xproto.KeyPressEvent{Detail: 10})
		if b != nil || act != ActionNone {
			t.Fatalf("keysym 0x%x: expected no output, got %q action %v", sym, string(b), act)
		}
	}
}

// testIntlKeyHandler builds a KeyHandler with the core mapping of a
// us-intl (FOUR_LEVEL, single group) layout, as the X server reports
// it: 7 keysyms per keycode, laid out [L0, L1, L0, L1, L2, L3, L2].
// Mod5 (0x80) is the Level3 (AltGr) modifier.
func testIntlKeyHandler() *KeyHandler {
	const per = 7
	syms := make([]xproto.Keysym, (256-8+1)*per)
	set := func(kc xproto.Keycode, row ...xproto.Keysym) {
		idx := int(kc-8) * per
		copy(syms[idx:idx+len(row)], row)
	}
	set(14, 0x35, 0x25, 0x35, 0x25, 0x20ac, 0xfe5b, 0x20ac)   // 5 / % / € / dead_cedilla
	set(26, 0x65, 0x45, 0x65, 0x45, 0xe9, 0xc9, 0xe9)         // e / E / é / É
	set(30, 0x75, 0x55, 0x75, 0x55, 0, 0, 0)                  // u / U
	set(48, 0xfe51, 0xfe57, 0xfe51, 0xfe57, 0x27, 0x22, 0x27) // dead_acute / dead_diaeresis / ' / "
	set(53, 0x78, 0x58, 0x78, 0x58, 0, 0, 0)                  // x / X
	set(54, 0x63, 0x43, 0x63, 0x43, 0, 0, 0)                  // c / C
	set(65, 0x20, 0x20, 0x20, 0x20, 0, 0, 0)                  // space
	return &KeyHandler{
		minKeycode:        8,
		maxKeycode:        255,
		keysymsPerKeycode: per,
		keysyms:           syms,
		numLockMask:       0x10,
		level3Mask:        0x80,
	}
}

func TestTranslateAltGrAndDeadKeys(t *testing.T) {
	const mod5 = 0x80 // Level3 / AltGr
	kh := testIntlKeyHandler()
	press := func(kc xproto.Keycode, state uint16) string {
		b, act := kh.Translate(xproto.KeyPressEvent{Detail: kc, State: state})
		if act != ActionNone {
			t.Fatalf("keycode %d state 0x%x: unexpected action %v", kc, state, act)
		}
		return string(b)
	}
	// Plain keys are unchanged.
	if got := press(26, 0); got != "e" {
		t.Fatalf("e: expected e, got %q", got)
	}
	if got := press(14, 0); got != "5" {
		t.Fatalf("5: expected 5, got %q", got)
	}
	if got := press(26, ModShift); got != "E" {
		t.Fatalf("Shift+e: expected E, got %q", got)
	}
	// AltGr selects the third level: EuroSign, é, apostrophe.
	if got := press(14, mod5); got != "€" {
		t.Fatalf("AltGr+5: expected €, got %q", got)
	}
	if got := press(26, mod5); got != "é" {
		t.Fatalf("AltGr+e: expected é, got %q", got)
	}
	if got := press(48, mod5); got != "'" {
		t.Fatalf("AltGr+apostrophe: expected ', got %q", got)
	}
	// AltGr+Shift selects the fourth level (dead_cedilla arms, emits nothing).
	if got := press(14, mod5|ModShift); got != "" {
		t.Fatalf("AltGr+Shift+5: expected nothing (dead_cedilla), got %q", got)
	}
	kh.pendingDead = 0

	// Dead-key composition.
	if got := press(48, 0); got != "" {
		t.Fatalf("dead_acute: expected nothing, got %q", got)
	}
	if got := press(26, 0); got != "é" {
		t.Fatalf("dead_acute+e: expected é, got %q", got)
	}
	if got := press(48, 0); got != "" {
		t.Fatalf("dead_acute: expected nothing, got %q", got)
	}
	if got := press(26, ModShift); got != "É" {
		t.Fatalf("dead_acute+Shift+e: expected É, got %q", got)
	}
	// No precomposed form: spacing accent + the character.
	if got := press(48, 0); got != "" {
		t.Fatalf("dead_acute: expected nothing, got %q", got)
	}
	if got := press(53, 0); got != "´x" {
		t.Fatalf("dead_acute+x: expected ´x, got %q", got)
	}
	// Dead key + space is the spacing accent.
	if got := press(48, 0); got != "" {
		t.Fatalf("dead_acute: expected nothing, got %q", got)
	}
	if got := press(65, 0); got != "´" {
		t.Fatalf("dead_acute+space: expected ´, got %q", got)
	}
	// Dead key + dead key: first one is emitted as spacing, second arms.
	if got := press(48, 0); got != "" {
		t.Fatalf("dead_acute: expected nothing, got %q", got)
	}
	if got := press(48, 0); got != "´" {
		t.Fatalf("dead_acute+dead_acute: expected ´, got %q", got)
	}
	if got := press(26, 0); got != "é" {
		t.Fatalf("dead_acute+e after double dead: expected é, got %q", got)
	}
	// Dead_diaeresis + u is ü.
	if got := press(48, ModShift); got != "" {
		t.Fatalf("dead_diaeresis: expected nothing, got %q", got)
	}
	if got := press(30, 0); got != "ü" {
		t.Fatalf("dead_diaeresis+u: expected ü, got %q", got)
	}
	// A modified key cancels the pending dead key.
	if got := press(48, 0); got != "" {
		t.Fatalf("dead_acute: expected nothing, got %q", got)
	}
	if got := press(54, ModCtrl); got != "\x03" {
		t.Fatalf("dead_acute then Ctrl+C: expected 0x03, got %q", got)
	}
	if got := press(26, 0); got != "e" {
		t.Fatalf("e after cancelled dead key: expected e, got %q", got)
	}
}

func TestTranslateAlt(t *testing.T) {
	// Alt+printable: ESC + character.
	kh := testKeyHandler([]xproto.Keysym{'b'})
	b, _ := kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModAlt})
	if string(b) != "\x1bb" {
		t.Fatalf("Alt+b: expected ESC b, got %q", string(b))
	}

	// Alt+Backspace: ESC + DEL.
	kh = testKeyHandler([]xproto.Keysym{0xff08})
	b, _ = kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModAlt})
	if string(b) != "\x1b\x7f" {
		t.Fatalf("Alt+Backspace: expected ESC DEL, got %q", string(b))
	}

	// Alt+Up: ESC + the arrow sequence.
	kh = testKeyHandler([]xproto.Keysym{0xff52})
	b, _ = kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModAlt})
	if string(b) != "\x1b\x1b[A" {
		t.Fatalf("Alt+Up: expected ESC ESC[A, got %q", string(b))
	}

	// Ctrl+Alt+a: ESC + control byte.
	kh = testKeyHandler([]xproto.Keysym{'a'})
	b, _ = kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModAlt | ModCtrl})
	if string(b) != "\x1b\x01" {
		t.Fatalf("Ctrl+Alt+a: expected ESC 0x01, got %q", string(b))
	}

	// Alt+digit tab switching is unchanged.
	kh = testKeyHandler([]xproto.Keysym{'3'})
	_, act := kh.Translate(xproto.KeyPressEvent{Detail: 10, State: ModAlt})
	if act != ActionSwitchTab3 {
		t.Fatalf("Alt+3: expected ActionSwitchTab3, got %v", act)
	}

	// Bare keys are unchanged by the Alt work.
	kh = testKeyHandler([]xproto.Keysym{'b'})
	b, _ = kh.Translate(xproto.KeyPressEvent{Detail: 10})
	if string(b) != "b" {
		t.Fatalf("b: expected b, got %q", string(b))
	}
}

func TestKeysymToRuneLegacyGreekCyrillic(t *testing.T) {
	// The two anchors from the bug report: the old formulas were 0x0F off.
	cases := map[xproto.Keysym]rune{
		0x06c1: 'а', // XK_Cyrillic_a -> U+0430
		0x07c1: 'Α', // XK_Greek_ALPHA -> U+0391
		0x06e1: 'А', // U+0410
		0x07e1: 'α', // U+03B1
		0x06a1: 'ђ', // U+0452
		0x07a1: 'Ά', // U+0386
		0x06ff: 'Ъ', // U+042A
		0x07f9: 'ω', // U+03C9
		0x06b0: '№', // U+2116
		0x07af: '―', // U+2015
	}
	for sym, want := range cases {
		r, ok := KeysymToRune(sym)
		if !ok || r != want {
			t.Fatalf("keysym 0x%x: expected %q, got %q ok=%v", sym, want, r, ok)
		}
	}

	// Unmapped legacy keysyms are dropped, not guessed.
	for _, sym := range []xproto.Keysym{0x06a0, 0x07d3, 0x07fe} {
		if r, ok := KeysymToRune(sym); ok {
			t.Fatalf("keysym 0x%x: expected no rune, got %q", sym, r)
		}
	}

	// Dead / ISO keysyms are not runes.
	for _, sym := range []xproto.Keysym{0xfe20, 0xfe50, 0xfe03, 0xfd00} {
		if r, ok := KeysymToRune(sym); ok {
			t.Fatalf("keysym 0x%x: expected no rune, got %q", sym, r)
		}
	}

	// Direct Unicode keysyms still work.
	r, ok := KeysymToRune(0x03bb)
	if !ok || r != 'λ' {
		t.Fatalf("expected lambda, got %q %v", r, ok)
	}
}

func TestIncrOutStateNextChunk(t *testing.T) {
	data := make([]byte, 200*1024)
	for i := range data {
		data[i] = byte(i)
	}
	st := &incrOutState{data: data, chunkSize: 64 * 1024}
	var got []byte
	for i := 0; i < 10; i++ {
		chunk, done := st.nextChunk()
		if done {
			if len(chunk) != 0 {
				t.Fatalf("final chunk must be empty, got %d bytes", len(chunk))
			}
			break
		}
		got = append(got, chunk...)
	}
	if len(got) != len(data) {
		t.Fatalf("expected %d bytes in chunks, got %d", len(data), len(got))
	}
	for i := range data {
		if got[i] != data[i] {
			t.Fatalf("chunked data mismatch at byte %d", i)
		}
	}

	// Empty data is done immediately.
	st = &incrOutState{data: nil, chunkSize: 64 * 1024}
	if _, done := st.nextChunk(); !done {
		t.Fatal("empty transfer must be done immediately")
	}
}
