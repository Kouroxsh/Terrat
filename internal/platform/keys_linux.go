//go:build linux

package platform

import (
	"fmt"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

type KeyHandler struct {
	minKeycode        xproto.Keycode
	maxKeycode        xproto.Keycode
	keysymsPerKeycode byte
	keysyms           []xproto.Keysym
	numLockMask       uint16
	modeSwitchMask    uint16
	xkbSupported      bool
	xkbOpcode         byte
	xkbBaseEvent      byte
	activeGroup       byte
}

// XKBStateNotifyEvent is delivered when the XKB keyboard layout/group changes.
type XKBStateNotifyEvent struct {
	Data []byte
}

func (e XKBStateNotifyEvent) Bytes() []byte  { return e.Data }
func (e XKBStateNotifyEvent) String() string { return "XKBStateNotify" }

func NewKeyHandler(X *xgb.Conn) (*KeyHandler, error) {
	setup := xproto.Setup(X)
	minKey := setup.MinKeycode
	maxKey := setup.MaxKeycode
	count := byte(maxKey - minKey + 1)

	mapping, err := xproto.GetKeyboardMapping(X, minKey, count).Reply()
	if err != nil {
		return nil, err
	}

	kh := &KeyHandler{
		minKeycode:        minKey,
		maxKeycode:        maxKey,
		keysymsPerKeycode: mapping.KeysymsPerKeycode,
		keysyms:           mapping.Keysyms,
		numLockMask:       0x10, // Mod2 default
	}
	kh.detectNumLockMask(X)
	kh.initXKB(X)
	return kh, nil
}

func (kh *KeyHandler) detectNumLockMask(X *xgb.Conn) {
	modMap, err := xproto.GetModifierMapping(X).Reply()
	if err != nil || modMap == nil {
		kh.numLockMask = 0x10
		return
	}
	kpm := int(modMap.KeycodesPerModifier)
	for mod := 0; mod < 8; mod++ {
		mask := uint16(1 << mod)
		for k := 0; k < kpm; k++ {
			kc := modMap.Keycodes[mod*kpm+k]
			if kc == 0 || kc < kh.minKeycode || kc > kh.maxKeycode {
				continue
			}
			idx := int(kc-kh.minKeycode) * int(kh.keysymsPerKeycode)
			for c := 0; c < int(kh.keysymsPerKeycode); c++ {
				if idx+c < len(kh.keysyms) {
					switch kh.keysyms[idx+c] {
					case 0xff7f: // XK_Num_Lock
						kh.numLockMask = mask
					case 0xff7e, 0xfe08, 0xfe09, 0xfe03: // Mode_switch / ISO_Group_Shift
						kh.modeSwitchMask = mask
					}
				}
			}
		}
	}
}

func (kh *KeyHandler) initXKB(X *xgb.Conn) {
	ext, err := xproto.QueryExtension(X, uint16(len("XKEYBOARD")), "XKEYBOARD").Reply()
	if err != nil || ext == nil || !ext.Present {
		return
	}
	kh.xkbOpcode = ext.MajorOpcode
	kh.xkbBaseEvent = ext.FirstEvent

	buf := make([]byte, 8)
	buf[0] = ext.MajorOpcode
	buf[1] = 0 // X_kbUseExtension
	xgb.Put16(buf[2:], 2)
	xgb.Put16(buf[4:], 1)
	xgb.Put16(buf[6:], 0)

	cookie := X.NewCookie(true, true)
	X.NewRequest(buf, cookie)
	reply, err := cookie.Reply()
	if err != nil || len(reply) < 8 || reply[1] == 0 {
		return
	}
	kh.xkbSupported = true

	reqSelect := make([]byte, 16)
	reqSelect[0] = ext.MajorOpcode
	reqSelect[1] = 1 // X_kbSelectEvents
	xgb.Put16(reqSelect[2:], 4)
	xgb.Put16(reqSelect[4:], 0x0100) // XkbUseCoreKbd
	xgb.Put16(reqSelect[6:], 0x0004) // affectWhich: XkbStateNotify (1<<2)
	xgb.Put16(reqSelect[8:], 0)
	xgb.Put16(reqSelect[10:], 0x0004) // selectAll: XkbStateNotify (1<<2)

	cookieSelect := X.NewCookie(false, false)
	X.NewRequest(reqSelect, cookieSelect)

	registerXKBEventFunc(ext.FirstEvent)

	kh.RefreshXKBState(X)
}

// xkbEventFuncsMu guards registration of the XKB event parser in the
// global xgb.NewEventFuncs map (L62): NewKeyHandler runs while the
// event loop reads that map to parse incoming events, so an unguarded
// write is a data race — and a second window would silently overwrite
// the first's handler with an identical one. The handler is
// stateless, so registering once is enough.
var xkbEventFuncsMu sync.Mutex

func registerXKBEventFunc(firstEvent byte) {
	xkbEventFuncsMu.Lock()
	defer xkbEventFuncsMu.Unlock()
	if _, ok := xgb.NewEventFuncs[int(firstEvent)]; ok {
		return
	}
	xgb.NewEventFuncs[int(firstEvent)] = func(b []byte) xgb.Event {
		return XKBStateNotifyEvent{Data: b}
	}
}

func (kh *KeyHandler) RefreshXKBState(X *xgb.Conn) {
	if !kh.xkbSupported || kh.xkbOpcode == 0 {
		return
	}
	reqState := make([]byte, 8)
	reqState[0] = kh.xkbOpcode
	reqState[1] = 4 // X_kbGetState
	xgb.Put16(reqState[2:], 2)
	xgb.Put16(reqState[4:], 0x0100) // XkbUseCoreKbd

	cookieState := X.NewCookie(true, true)
	X.NewRequest(reqState, cookieState)
	replyState, err := cookieState.Reply()
	if err == nil && len(replyState) >= 14 {
		kh.activeGroup = replyState[12] & 3
	}
}

func (kh *KeyHandler) SetGroup(group byte) {
	kh.activeGroup = group & 3
}

// keysymAt returns the keysym at row idx, column c, or 0 when the
// keyboard mapping reply was short for that keycode. A truncated
// GetKeyboardMapping reply used to panic the event-loop goroutine on a
// bare kh.keysyms[idx+col] (L49) — every row access below goes through
// here, so a short reply degrades to "unknown key" instead of a crash.
func (kh *KeyHandler) keysymAt(idx, c int) xproto.Keysym {
	if idx+c < 0 || idx+c >= len(kh.keysyms) {
		return 0
	}
	return kh.keysyms[idx+c]
}

func (kh *KeyHandler) KeySym(ev xproto.KeyPressEvent) xproto.Keysym {
	keycode := ev.Detail
	if keycode < kh.minKeycode || keycode > kh.maxKeycode {
		return 0
	}
	idx := int(keycode-kh.minKeycode) * int(kh.keysymsPerKeycode)
	if idx >= len(kh.keysyms) {
		return 0
	}

	// In X11, bits 13-14 specify keyboard group (0-3) for layout switching
	group := int((ev.State >> 13) & 3)
	if group == 0 && kh.modeSwitchMask != 0 && (ev.State&kh.modeSwitchMask) != 0 {
		group = 1
	}
	if group == 0 && kh.activeGroup != 0 {
		group = int(kh.activeGroup)
	}

	col := group * 2
	if col >= int(kh.keysymsPerKeycode) {
		col = 0
	}

	isShift := (ev.State & ModShift) != 0
	isLock := (ev.State & ModLock) != 0 // CapsLock

	isKP := false
	for c := 0; c < int(kh.keysymsPerKeycode); c++ {
		s := kh.keysymAt(idx, c)
		if (s >= 0xff80 && s <= 0xffbd) || (s >= 0xffb0 && s <= 0xffb9) {
			isKP = true
			break
		}
	}

	keysym := kh.keysymAt(idx, col)
	shiftCol := col + 1

	if isKP {
		isNumLock := (ev.State & kh.numLockMask) != 0
		effectiveShift := isShift != isNumLock
		if effectiveShift && shiftCol < int(kh.keysymsPerKeycode) && kh.keysymAt(idx, shiftCol) != 0 {
			keysym = kh.keysymAt(idx, shiftCol)
		}
	} else {
		if shiftCol < int(kh.keysymsPerKeycode) && kh.keysymAt(idx, shiftCol) != 0 {
			if isShift {
				keysym = kh.keysymAt(idx, shiftCol)
			}
		}
	}

	// Fallback to Group 0 if current group has no keysym
	if keysym == 0 && col != 0 {
		keysym = kh.keysymAt(idx, 0)
		if isShift && kh.keysymsPerKeycode > 1 && kh.keysymAt(idx, 1) != 0 {
			keysym = kh.keysymAt(idx, 1)
		}
	}

	// Apply CapsLock (L40): fold the case of the letter the keysym
	// stands for, not just ASCII. Latin-1 and direct-Unicode keysyms
	// equal their code points, so unicode.ToUpper/ToLower applies
	// directly; legacy Greek/Cyrillic keysyms go through the lookup
	// table in reverse — their case pairs are non-linear (0x07b1 ά ↔
	// 0x07a1 Ά), so no arithmetic on the keysym can fold them.
	if isLock {
		if r, ok := KeysymToRune(keysym); ok && unicode.IsLetter(r) {
			var folded rune
			if isShift {
				folded = unicode.ToLower(r)
			} else {
				folded = unicode.ToUpper(r)
			}
			if folded != r {
				if (keysym >= 0x06a0 && keysym <= 0x06ff) || (keysym >= 0x07a1 && keysym <= 0x07fe) {
					if ks, ok := legacyKeysymByRune[folded]; ok {
						keysym = ks
					}
				} else {
					keysym = xproto.Keysym(folded)
				}
			}
		}
	}

	return keysym
}

// legacyKeysymByRune is the reverse of legacyGreekCyrillicKeysyms: the
// keysym for a rune, for CapsLock folding (L40). A rune with two legacy
// keysyms keeps the first — both produce the same rune, so it does not
// matter which one is picked.
var legacyKeysymByRune = func() map[rune]xproto.Keysym {
	m := make(map[rune]xproto.Keysym, len(legacyGreekCyrillicKeysyms))
	for ks, r := range legacyGreekCyrillicKeysyms {
		if _, exists := m[r]; !exists {
			m[r] = ks
		}
	}
	return m
}()

func (kh *KeyHandler) RefreshMapping(X *xgb.Conn) error {
	setup := xproto.Setup(X)
	minKey := setup.MinKeycode
	maxKey := setup.MaxKeycode
	count := byte(maxKey - minKey + 1)

	mapping, err := xproto.GetKeyboardMapping(X, minKey, count).Reply()
	if err != nil {
		return err
	}
	kh.minKeycode = minKey
	kh.maxKeycode = maxKey
	kh.keysymsPerKeycode = mapping.KeysymsPerKeycode
	kh.keysyms = mapping.Keysyms
	kh.detectNumLockMask(X)
	kh.RefreshXKBState(X)
	return nil
}

// funcKeySeq returns the xterm sequence for F1-F20 (L41). Modified keys
// encode Shift/Alt/Ctrl as a `;{mod}` parameter (mod = 1 + shift + 2*alt
// + 4*ctrl): Shift+F1 is ESC[1;2P, Ctrl+F5 is ESC[15;5~. Unmodified keys
// keep their classic form (SS3 for F1-F4, tilde for F5+). F13-F20 use the
// standard tilde codes (25, 26, 28, 29, 31-34); F21+ have no standard
// xterm encoding and are still dropped. The Alt modifier lives in the
// parameter, so callers must not also prepend an ESC (see Translate).
func funcKeySeq(keysym xproto.Keysym, isShift, isAlt, isCtrl bool) []byte {
	mod := 1
	if isShift {
		mod += 1
	}
	if isAlt {
		mod += 2
	}
	if isCtrl {
		mod += 4
	}
	param := ""
	if mod > 1 {
		param = fmt.Sprintf(";%d", mod)
	}
	switch keysym {
	case 0xffbe: // F1
		if mod > 1 {
			return []byte("\x1b[1" + param + "P")
		}
		return []byte("\x1bOP")
	case 0xffbf: // F2
		if mod > 1 {
			return []byte("\x1b[1" + param + "Q")
		}
		return []byte("\x1bOQ")
	case 0xffc0: // F3
		if mod > 1 {
			return []byte("\x1b[1" + param + "R")
		}
		return []byte("\x1bOR")
	case 0xffc1: // F4
		if mod > 1 {
			return []byte("\x1b[1" + param + "S")
		}
		return []byte("\x1bOS")
	}
	tilde := map[xproto.Keysym]int{
		0xffc2: 15, 0xffc3: 17, 0xffc4: 18, 0xffc5: 19, 0xffc6: 20,
		0xffc7: 21, 0xffc8: 23, 0xffc9: 24,
		0xffca: 25, 0xffcb: 26, 0xffcc: 28, 0xffcd: 29, 0xffce: 31,
		0xffcf: 32, 0xffd0: 33, 0xffd1: 34,
	}
	if n, ok := tilde[keysym]; ok {
		return []byte(fmt.Sprintf("\x1b[%d%s~", n, param))
	}
	return nil
}

func (kh *KeyHandler) Translate(ev xproto.KeyPressEvent, flags ...bool) ([]byte, ActionType) {
	keysym := kh.KeySym(ev)
	if keysym == 0 {
		return nil, ActionNone
	}

	// flags[0] is application-cursor mode; flags[1], when present,
	// reports whether the terminal is on the alternate screen (L42).
	isAppCursor := len(flags) > 0 && flags[0]
	isAltScreen := len(flags) > 1 && flags[1]
	state := ev.State
	isShift := (state & ModShift) != 0
	isCtrl := (state & ModCtrl) != 0
	isAlt := (state & ModAlt) != 0

	// ISO_Left_Tab (0xfe20) is what XKB reports for Shift+Tab, so the 0xff09
	// case below never sees it. Handle it here, before the dead-key filter.
	if keysym == 0xfe20 {
		if isCtrl && isShift {
			return nil, ActionPrevTab
		}
		if isAlt {
			return []byte("\x1b\x1b[Z"), ActionNone
		}
		return []byte("\x1b[Z"), ActionNone
	}

	// Dead keys (0xfe50+), compose/multi-key and the other ISO function
	// keysyms (0xfe00-0xfeff) and the 3270 range (0xfd00-0xfdff) are not
	// characters: drop them silently instead of leaking their code points
	// as UTF-8 into the PTY. AltGr (0xfe03) and Mode_switch (0xfe08/0xfe09)
	// are modifiers, so the key press itself must also produce nothing.
	if keysym >= 0xfd00 && keysym <= 0xfeff {
		return nil, ActionNone
	}

	if isAlt && !isCtrl && keysym >= '1' && keysym <= '9' {
		return nil, ActionSwitchTab1 + ActionType(keysym-'1')
	}

	if isCtrl && !isShift {
		if keysym == '=' || keysym == '+' {
			return nil, ActionZoomIn
		}
		if keysym == '-' || keysym == '_' {
			return nil, ActionZoomOut
		}
		if keysym == '0' {
			return nil, ActionZoomReset
		}
	}
	if isCtrl && isShift {
		if keysym == '+' || keysym == '=' {
			return nil, ActionZoomIn
		}
		if keysym == '_' || keysym == '-' {
			return nil, ActionZoomOut
		}
	}

	if isCtrl {
		if keysym == ',' || keysym == '<' {
			return nil, ActionPreferences
		}
		if isShift && (keysym == 'P' || keysym == 'p') {
			return nil, ActionPreferences
		}
	}

	if isCtrl && isShift {
		if keysym == 'T' || keysym == 't' {
			return nil, ActionNewTab
		}
		if keysym == 'W' || keysym == 'w' {
			return nil, ActionCloseTab
		}
		if keysym == 'F' || keysym == 'f' {
			return nil, ActionSearch
		}
		if keysym == 'D' || keysym == 'd' {
			return nil, ActionToggleDiagnostics
		}
		if keysym == 0xff09 || keysym == 0xfe20 {
			return nil, ActionPrevTab
		}
		if keysym == 0xff55 {
			return nil, ActionPrevTab
		}
		if keysym == 0xff56 {
			return nil, ActionNextTab
		}
	}

	if isCtrl && !isShift {
		if keysym == 0xff09 {
			return nil, ActionNextTab
		}
		if keysym == 0xff55 {
			return nil, ActionPrevTab
		}
		if keysym == 0xff56 {
			return nil, ActionNextTab
		}
	}

	// L42: Shift+PgUp/PgDn/Home/End scroll the terminal — but only on
	// the normal screen. Inside a fullscreen app (vim, less, ...) the
	// keys must reach the app instead; the shifted sequences are emitted
	// by the switch below.
	if !isAltScreen {
		if (keysym == 0xff55 || keysym == 0xff9a) && isShift {
			return nil, ActionScrollUp
		}
		if (keysym == 0xff56 || keysym == 0xff9b) && isShift {
			return nil, ActionScrollDown
		}
		if (keysym == 0xff50 || keysym == 0xff95) && isShift {
			return nil, ActionScrollTop
		}
		if (keysym == 0xff57 || keysym == 0xff9c) && isShift {
			return nil, ActionScrollBottom
		}
	}

	if isCtrl && isShift {
		if keysym == 'C' || keysym == 'c' {
			return nil, ActionCopy
		}
		if keysym == 'V' || keysym == 'v' {
			return nil, ActionPaste
		}
		if keysym == 'A' || keysym == 'a' {
			return nil, ActionSelectAll
		}
	}
	if isShift && keysym == 0xff63 {
		return nil, ActionPaste
	}

	if isCtrl {
		// Ctrl+letter / Ctrl+symbol produce a control byte; with Alt held
		// too, prefix ESC so the Alt modifier is not lost (Ctrl+Alt+a is
		// ESC 0x01, as readline/Emacs expect).
		var ctrl []byte
		if keysym >= 'a' && keysym <= 'z' {
			ctrl = []byte{byte(keysym - 'a' + 1)}
		} else if keysym >= 'A' && keysym <= 'Z' {
			ctrl = []byte{byte(keysym - 'A' + 1)}
		} else {
			switch keysym {
			case '@':
				ctrl = []byte{0x00}
			case '[':
				ctrl = []byte{0x1b}
			case '\\':
				ctrl = []byte{0x1c}
			case ']':
				ctrl = []byte{0x1d}
			case '^':
				ctrl = []byte{0x1e}
			case '_':
				ctrl = []byte{0x1f}
			// L39: Ctrl+Space is NUL (0x00), not a space — the old code
			// fell through and sent 0x20. Ctrl+/ is 0x1f, not 0x2f.
			case ' ':
				ctrl = []byte{0x00}
			case '/':
				ctrl = []byte{0x1f}
			}
		}
		if ctrl != nil {
			if isAlt {
				return append([]byte{0x1b}, ctrl...), ActionNone
			}
			return ctrl, ActionNone
		}
	}

	// L41: function keys, with modifiers encoded (see funcKeySeq). This
	// runs before the sequence switch so the Alt modifier is not also
	// prepended as an ESC by the switch's trailing Alt handling.
	if seq := funcKeySeq(keysym, isShift, isAlt, isCtrl); seq != nil {
		return seq, ActionNone
	}

	var seq []byte
	switch keysym {
	// Keypad numbers (0-9)
	case 0xffb0, 0xffb1, 0xffb2, 0xffb3, 0xffb4, 0xffb5, 0xffb6, 0xffb7, 0xffb8, 0xffb9:
		seq = []byte{byte('0' + (keysym - 0xffb0))}

	// Keypad arithmetic and separators
	case 0xffaa: // KP_Multiply
		seq = []byte("*")
	case 0xffab: // KP_Add
		seq = []byte("+")
	case 0xffac: // KP_Separator
		seq = []byte(",")
	case 0xffad: // KP_Subtract
		seq = []byte("-")
	case 0xffae: // KP_Decimal
		seq = []byte(".")
	case 0xffaf: // KP_Divide
		seq = []byte("/")
	case 0xffbd: // KP_Equal
		seq = []byte("=")
	case 0xff80: // KP_Space
		seq = []byte(" ")
	case 0xff89: // KP_Tab
		seq = []byte("\t")

	// Keypad navigation (when NumLock is OFF)
	case 0xff95: // KP_Home
		if isShift {
			// L42: shifted Home in a fullscreen app.
			seq = []byte("\x1b[1;2H")
		} else if isAppCursor {
			seq = []byte("\x1bOH")
		} else {
			seq = []byte("\x1b[H")
		}
	case 0xff96: // KP_Left
		if isAppCursor {
			seq = []byte("\x1bOD")
		} else {
			seq = []byte("\x1b[D")
		}
	case 0xff97: // KP_Up
		if isAppCursor {
			seq = []byte("\x1bOA")
		} else {
			seq = []byte("\x1b[A")
		}
	case 0xff98: // KP_Right
		if isAppCursor {
			seq = []byte("\x1bOC")
		} else {
			seq = []byte("\x1b[C")
		}
	case 0xff99: // KP_Down
		if isAppCursor {
			seq = []byte("\x1bOB")
		} else {
			seq = []byte("\x1b[B")
		}
	case 0xff9a: // KP_Prior (Page Up)
		if isShift {
			// L42: shifted Page Up in a fullscreen app.
			seq = []byte("\x1b[5;2~")
		} else {
			seq = []byte("\x1b[5~")
		}
	case 0xff9b: // KP_Next (Page Down)
		if isShift {
			// L42: shifted Page Down in a fullscreen app.
			seq = []byte("\x1b[6;2~")
		} else {
			seq = []byte("\x1b[6~")
		}
	case 0xff9c: // KP_End
		if isShift {
			// L42: shifted End in a fullscreen app.
			seq = []byte("\x1b[1;2F")
		} else if isAppCursor {
			seq = []byte("\x1bOF")
		} else {
			seq = []byte("\x1b[F")
		}
	case 0xff9d: // KP_Begin (Center 5)
		seq = []byte("\x1b[E")
	case 0xff9e: // KP_Insert
		seq = []byte("\x1b[2~")
	case 0xff9f: // KP_Delete
		seq = []byte("\x1b[3~")
	case 0xff0d, 0xff8d:
		seq = []byte("\r")
	case 0xff08:
		seq = []byte("\x7f")
	case 0xff09:
		if isShift {
			seq = []byte("\x1b[Z")
		} else {
			seq = []byte("\t")
		}
	case 0xff1b:
		seq = []byte("\x1b")
	case 0xffff:
		seq = []byte("\x1b[3~")
	case 0xff63:
		seq = []byte("\x1b[2~")
	case 0xff50:
		if isShift {
			// L42: shifted Home in a fullscreen app.
			seq = []byte("\x1b[1;2H")
		} else if isAppCursor {
			seq = []byte("\x1bOH")
		} else {
			seq = []byte("\x1b[H")
		}
	case 0xff57:
		if isShift {
			// L42: shifted End in a fullscreen app.
			seq = []byte("\x1b[1;2F")
		} else if isAppCursor {
			seq = []byte("\x1bOF")
		} else {
			seq = []byte("\x1b[F")
		}
	case 0xff55:
		if isShift {
			// L42: shifted Page Up in a fullscreen app.
			seq = []byte("\x1b[5;2~")
		} else {
			seq = []byte("\x1b[5~")
		}
	case 0xff56:
		if isShift {
			// L42: shifted Page Down in a fullscreen app.
			seq = []byte("\x1b[6;2~")
		} else {
			seq = []byte("\x1b[6~")
		}
	case 0xff52:
		if isAppCursor {
			seq = []byte("\x1bOA")
		} else {
			seq = []byte("\x1b[A")
		}
	case 0xff54:
		if isAppCursor {
			seq = []byte("\x1bOB")
		} else {
			seq = []byte("\x1b[B")
		}
	case 0xff53:
		if isAppCursor {
			seq = []byte("\x1bOC")
		} else {
			seq = []byte("\x1b[C")
		}
	case 0xff51:
		if isAppCursor {
			seq = []byte("\x1bOD")
		} else {
			seq = []byte("\x1b[D")
		}

		// NB: F1-F20 are handled by funcKeySeq before this switch (L41).
	}

	if seq != nil {
		// Alt+special key: ESC plus the key's sequence (Alt+Up is ESC
		// ESC[A), so applications can tell it apart from the bare key.
		if isAlt {
			return append([]byte{0x1b}, seq...), ActionNone
		}
		return seq, ActionNone
	}

	if r, ok := KeysymToRune(keysym); ok {
		buf := make([]byte, 4)
		n := utf8.EncodeRune(buf, r)
		if isAlt {
			// Alt+printable key: ESC followed by the character, so the
			// application sees the Alt modifier (Alt+b/f/d in readline,
			// Alt+Backspace, ...).
			return append([]byte{0x1b}, buf[:n]...), ActionNone
		}
		return buf[:n], ActionNone
	}

	return nil, ActionNone
}

// legacyGreekCyrillicKeysyms maps the legacy X11 Greek/Cyrillic keysyms
// (0x06a1-0x06ff, 0x07a1-0x07f9) to their Unicode runes, transcribed from
// X11/keysymdef.h. The correspondence is non-linear, so no formula can
// replace this table: e.g. XK_Cyrillic_a 0x06c1 is U+0430 and
// XK_Greek_ALPHA 0x07c1 is U+0391. Modern XKB layouts report these letters
// as direct Unicode keysyms (0x03xx/0x04xx or 0x01000000-based) and never
// reach this table; it only serves old servers/layouts.
var legacyGreekCyrillicKeysyms = map[xproto.Keysym]rune{
	0x06a1: '\u0452', 0x06a2: '\u0453', 0x06a3: '\u0451', 0x06a4: '\u0454', 0x06a5: '\u0455',
	0x06a6: '\u0456', 0x06a7: '\u0457', 0x06a8: '\u0458', 0x06a9: '\u0459', 0x06aa: '\u045a',
	0x06ab: '\u045b', 0x06ac: '\u045c', 0x06ad: '\u0491', 0x06ae: '\u045e', 0x06af: '\u045f',
	0x06b0: '\u2116', 0x06b1: '\u0402', 0x06b2: '\u0403', 0x06b3: '\u0401', 0x06b4: '\u0404',
	0x06b5: '\u0405', 0x06b6: '\u0406', 0x06b7: '\u0407', 0x06b8: '\u0408', 0x06b9: '\u0409',
	0x06ba: '\u040a', 0x06bb: '\u040b', 0x06bc: '\u040c', 0x06bd: '\u0490', 0x06be: '\u040e',
	0x06bf: '\u040f', 0x06c0: '\u044e', 0x06c1: '\u0430', 0x06c2: '\u0431', 0x06c3: '\u0446',
	0x06c4: '\u0434', 0x06c5: '\u0435', 0x06c6: '\u0444', 0x06c7: '\u0433', 0x06c8: '\u0445',
	0x06c9: '\u0438', 0x06ca: '\u0439', 0x06cb: '\u043a', 0x06cc: '\u043b', 0x06cd: '\u043c',
	0x06ce: '\u043d', 0x06cf: '\u043e', 0x06d0: '\u043f', 0x06d1: '\u044f', 0x06d2: '\u0440',
	0x06d3: '\u0441', 0x06d4: '\u0442', 0x06d5: '\u0443', 0x06d6: '\u0436', 0x06d7: '\u0432',
	0x06d8: '\u044c', 0x06d9: '\u044b', 0x06da: '\u0437', 0x06db: '\u0448', 0x06dc: '\u044d',
	0x06dd: '\u0449', 0x06de: '\u0447', 0x06df: '\u044a', 0x06e0: '\u042e', 0x06e1: '\u0410',
	0x06e2: '\u0411', 0x06e3: '\u0426', 0x06e4: '\u0414', 0x06e5: '\u0415', 0x06e6: '\u0424',
	0x06e7: '\u0413', 0x06e8: '\u0425', 0x06e9: '\u0418', 0x06ea: '\u0419', 0x06eb: '\u041a',
	0x06ec: '\u041b', 0x06ed: '\u041c', 0x06ee: '\u041d', 0x06ef: '\u041e', 0x06f0: '\u041f',
	0x06f1: '\u042f', 0x06f2: '\u0420', 0x06f3: '\u0421', 0x06f4: '\u0422', 0x06f5: '\u0423',
	0x06f6: '\u0416', 0x06f7: '\u0412', 0x06f8: '\u042c', 0x06f9: '\u042b', 0x06fa: '\u0417',
	0x06fb: '\u0428', 0x06fc: '\u042d', 0x06fd: '\u0429', 0x06fe: '\u0427', 0x06ff: '\u042a',
	0x07a1: '\u0386', 0x07a2: '\u0388', 0x07a3: '\u0389', 0x07a4: '\u038a', 0x07a5: '\u03aa',
	0x07a6: '\u0390', 0x07a7: '\u038c', 0x07a8: '\u038e', 0x07a9: '\u03ab', 0x07aa: '\u03b0',
	0x07ab: '\u038f', 0x07ae: '\u0385', 0x07af: '\u2015', 0x07b1: '\u03ac', 0x07b2: '\u03ad',
	0x07b3: '\u03ae', 0x07b4: '\u03af', 0x07b5: '\u03ca', 0x07b6: '\u0390', 0x07b7: '\u03cc',
	0x07b8: '\u03cd', 0x07b9: '\u03cb', 0x07ba: '\u03b0', 0x07bb: '\u03ce', 0x07c1: '\u0391',
	0x07c2: '\u0392', 0x07c3: '\u0393', 0x07c4: '\u0394', 0x07c5: '\u0395', 0x07c6: '\u0396',
	0x07c7: '\u0397', 0x07c8: '\u0398', 0x07c9: '\u0399', 0x07ca: '\u039a', 0x07cb: '\u039b',
	0x07cc: '\u039c', 0x07cd: '\u039d', 0x07ce: '\u039e', 0x07cf: '\u039f', 0x07d0: '\u03a0',
	0x07d1: '\u03a1', 0x07d2: '\u03a3', 0x07d4: '\u03a4', 0x07d5: '\u03a5', 0x07d6: '\u03a6',
	0x07d7: '\u03a7', 0x07d8: '\u03a8', 0x07d9: '\u03a9', 0x07e1: '\u03b1', 0x07e2: '\u03b2',
	0x07e3: '\u03b3', 0x07e4: '\u03b4', 0x07e5: '\u03b5', 0x07e6: '\u03b6', 0x07e7: '\u03b7',
	0x07e8: '\u03b8', 0x07e9: '\u03b9', 0x07ea: '\u03ba', 0x07eb: '\u03bb', 0x07ec: '\u03bc',
	0x07ed: '\u03bd', 0x07ee: '\u03be', 0x07ef: '\u03bf', 0x07f0: '\u03c0', 0x07f1: '\u03c1',
	0x07f2: '\u03c3', 0x07f3: '\u03c2', 0x07f4: '\u03c4', 0x07f5: '\u03c5', 0x07f6: '\u03c6',
	0x07f7: '\u03c7', 0x07f8: '\u03c8', 0x07f9: '\u03c9',
}

// KeysymToRune converts an X11 keysym to its corresponding Unicode rune.
func KeysymToRune(keysym xproto.Keysym) (rune, bool) {
	// Keypad numbers (0-9)
	if keysym >= 0xffb0 && keysym <= 0xffb9 {
		return rune('0' + (keysym - 0xffb0)), true
	}

	// Keypad arithmetic and special
	switch keysym {
	case 0xffaa:
		return '*', true
	case 0xffab:
		return '+', true
	case 0xffac:
		return ',', true
	case 0xffad:
		return '-', true
	case 0xffae:
		return '.', true
	case 0xffaf:
		return '/', true
	case 0xffbd:
		return '=', true
	case 0xff80:
		return ' ', true
	case 0xff89:
		return '\t', true
	case 0xff8d:
		return '\r', true
	}
	// Standard ASCII printable
	if keysym >= 0x0020 && keysym <= 0x007e {
		return rune(keysym), true
	}

	// Latin-1 Supplement
	if keysym >= 0x00a0 && keysym <= 0x00ff {
		return rune(keysym), true
	}

	// Direct Unicode keysyms (0x01000000 to 0x0110ffff)
	if (keysym & 0xff000000) == 0x01000000 {
		cp := rune(keysym & 0x00ffffff)
		if cp >= 0x20 && cp <= 0x10ffff {
			return cp, true
		}
	}

	// Dead keys, compose keys and the other ISO function keysyms
	// (0xfd00-0xfeff) are not characters: never emit them as runes.
	// (ISO_Left_Tab 0xfe20 is Shift+Tab and is handled in Translate
	// before this function is reached.)
	if keysym >= 0xfd00 && keysym <= 0xfeff {
		return 0, false
	}

	// Legacy Greek/Cyrillic keysyms: look them up, never guess. A keysym in
	// these ranges that is missing from the table is unmapped (e.g. 0x06a0,
	// 0x07d3) and is dropped rather than mapped to a wrong character.
	if r, ok := legacyGreekCyrillicKeysyms[keysym]; ok {
		return r, true
	}
	if (keysym >= 0x06a0 && keysym <= 0x06ff) || (keysym >= 0x07a1 && keysym <= 0x07fe) {
		return 0, false
	}

	// Unicode in standard range >= 0x0100 (excluding standard X11 special function keys 0xff00-0xffff)
	// Modern XKB layouts (including Cyrillic and Greek) often map directly to Unicode codepoints
	if keysym >= 0x0100 && keysym <= 0x10ffff && (keysym < 0xff00 || keysym > 0xffff) {
		return rune(keysym), true
	}

	return 0, false
}
