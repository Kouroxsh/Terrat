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

	level3Mask   uint16
	pendingDead  xproto.Keysym
	xkbSupported bool
	xkbOpcode    byte
	xkbBaseEvent byte
	activeGroup  byte
}

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
		numLockMask:       0x10,
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
					case 0xff7f:
						kh.numLockMask = mask
					case 0xff7e, 0xfe08, 0xfe09:
						kh.modeSwitchMask = mask
					case 0xfe03:

						kh.level3Mask = mask
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
	buf[1] = 0
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
	reqSelect[1] = 1
	xgb.Put16(reqSelect[2:], 4)
	xgb.Put16(reqSelect[4:], 0x0100)

	xgb.Put16(reqSelect[6:], 0x0006)
	xgb.Put16(reqSelect[8:], 0)
	xgb.Put16(reqSelect[10:], 0x0006)

	cookieSelect := X.NewCookie(false, false)
	X.NewRequest(reqSelect, cookieSelect)

	registerXKBEventFunc(ext.FirstEvent)

	kh.RefreshXKBState(X)
}

var xkbEventFuncsMu sync.Mutex

func registerXKBEventFunc(firstEvent byte) {
	xkbEventFuncsMu.Lock()
	defer xkbEventFuncsMu.Unlock()

	for kind := 0; kind <= 2; kind++ {
		code := int(firstEvent) + kind
		if _, ok := xgb.NewEventFuncs[code]; ok {
			continue
		}
		xgb.NewEventFuncs[code] = func(b []byte) xgb.Event {
			return XKBStateNotifyEvent{Data: b}
		}
	}
}

func (kh *KeyHandler) RefreshXKBState(X *xgb.Conn) {
	if !kh.xkbSupported || kh.xkbOpcode == 0 {
		return
	}
	reqState := make([]byte, 8)
	reqState[0] = kh.xkbOpcode
	reqState[1] = 4
	xgb.Put16(reqState[2:], 2)
	xgb.Put16(reqState[4:], 0x0100)

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

	group := int((ev.State >> 13) & 3)
	if group == 0 && kh.modeSwitchMask != 0 && (ev.State&kh.modeSwitchMask) != 0 {
		group = 1
	}
	if group == 0 && kh.activeGroup != 0 {
		group = int(kh.activeGroup)
	}

	coreIndex := func(group, level int) int {
		if level < 2 {
			return group*2 + level
		}
		return 4 + group*2 + (level - 2)
	}

	isShift := (ev.State & ModShift) != 0
	isLock := (ev.State & ModLock) != 0
	isLevel3 := kh.level3Mask != 0 && (ev.State&kh.level3Mask) != 0

	isKP := false
	for c := 0; c < int(kh.keysymsPerKeycode); c++ {
		s := kh.keysymAt(idx, c)
		if (s >= 0xff80 && s <= 0xffbd) || (s >= 0xffb0 && s <= 0xffb9) {
			isKP = true
			break
		}
	}

	level := 0
	if isKP {
		isNumLock := (ev.State & kh.numLockMask) != 0
		if isShift != isNumLock {
			level = 1
		}
	} else {
		if isShift {
			level |= 1
		}
		if isLevel3 {
			level |= 2
		}
	}

	keysym := kh.keysymAt(idx, coreIndex(group, level))

	if keysym == 0 {
		for _, l := range []int{level & 1, level & 2, 0} {
			if l == level {
				continue
			}
			if keysym = kh.keysymAt(idx, coreIndex(group, l)); keysym != 0 {
				break
			}
		}
	}

	if keysym == 0 && group != 0 {
		for _, l := range []int{level, level & 1, level & 2, 0} {
			if keysym = kh.keysymAt(idx, coreIndex(0, l)); keysym != 0 {
				break
			}
		}
	}

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
	case 0xffbe:
		if mod > 1 {
			return []byte("\x1b[1" + param + "P")
		}
		return []byte("\x1bOP")
	case 0xffbf:
		if mod > 1 {
			return []byte("\x1b[1" + param + "Q")
		}
		return []byte("\x1bOQ")
	case 0xffc0:
		if mod > 1 {
			return []byte("\x1b[1" + param + "R")
		}
		return []byte("\x1bOR")
	case 0xffc1:
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

func encodeRune(r rune) []byte {
	buf := make([]byte, 4)
	n := utf8.EncodeRune(buf, r)
	return buf[:n]
}

func isDeadKeysym(keysym xproto.Keysym) bool {
	switch keysym {
	case 0xfe50, 0xfe51, 0xfe52, 0xfe53, 0xfe54, 0xfe55, 0xfe56, 0xfe57,
		0xfe58, 0xfe59, 0xfe5a, 0xfe5b, 0xfe5c, 0xfe60, 0xfe61, 0xfe62,
		0xfe63, 0xfe64, 0xfe65, 0xfe66, 0xfe6e:
		return true
	}
	return false
}

func deadSpacing(keysym xproto.Keysym) rune {
	switch keysym {
	case 0xfe50:
		return '\u0060'
	case 0xfe51:
		return '\u00b4'
	case 0xfe52:
		return '\u005e'
	case 0xfe53:
		return '\u007e'
	case 0xfe54:
		return '\u00af'
	case 0xfe55:
		return '\u02d8'
	case 0xfe56:
		return '\u02d9'
	case 0xfe57:
		return '\u00a8'
	case 0xfe58:
		return '\u02da'
	case 0xfe59:
		return '\u02dd'
	case 0xfe5a:
		return '\u02c7'
	case 0xfe5b:
		return '\u00b8'
	case 0xfe5c:
		return '\u02db'
	case 0xfe60:
		return '\u002e'
	case 0xfe61:
		return '\u02bc'
	case 0xfe62:
		return '\u02bc'
	case 0xfe63:
		return '\u002f'
	case 0xfe64:
		return '\u02bc'
	case 0xfe65:
		return '\u02bd'
	case 0xfe66:
		return '\u02f5'
	case 0xfe6e:
		return '\u002c'
	}
	return ' '
}

var deadCompose = map[xproto.Keysym]map[rune]rune{
	0xfe50: {
		'a': '\u00e0', 'e': '\u00e8', 'i': '\u00ec', 'o': '\u00f2', 'u': '\u00f9',
		'A': '\u00c0', 'E': '\u00c8', 'I': '\u00cc', 'O': '\u00d2', 'U': '\u00d9',
		'n': '\u01f9', 'N': '\u01f8', ' ': '\u0060',
	},
	0xfe51: {
		'a': '\u00e1', 'e': '\u00e9', 'i': '\u00ed', 'o': '\u00f3', 'u': '\u00fa', 'y': '\u00fd',
		'A': '\u00c1', 'E': '\u00c9', 'I': '\u00cd', 'O': '\u00d3', 'U': '\u00da', 'Y': '\u00dd',
		'c': '\u0107', 'C': '\u0106', 'n': '\u0144', 'N': '\u0143', 's': '\u015b', 'S': '\u015a',
		'z': '\u017a', 'Z': '\u0179', 'l': '\u013a', 'L': '\u0139', 'r': '\u0155', 'R': '\u0154',
		'g': '\u01f5', 'G': '\u01f4', ' ': '\u00b4',
	},
	0xfe52: {
		'a': '\u00e2', 'e': '\u00ea', 'i': '\u00ee', 'o': '\u00f4', 'u': '\u00fb',
		'A': '\u00c2', 'E': '\u00ca', 'I': '\u00ce', 'O': '\u00d4', 'U': '\u00db',
		'c': '\u0109', 'C': '\u0108', 'g': '\u011d', 'G': '\u011c', 'h': '\u0125', 'H': '\u0124',
		'j': '\u0135', 'J': '\u0134', 's': '\u015d', 'S': '\u015c', 'w': '\u0175', 'W': '\u0174',
		'y': '\u0177', 'Y': '\u0176', ' ': '\u005e',
	},
	0xfe53: {
		'a': '\u00e3', 'e': '\u1ebd', 'i': '\u0129', 'o': '\u00f5', 'u': '\u0169',
		'A': '\u00c3', 'E': '\u1ebc', 'I': '\u0128', 'O': '\u00d5', 'U': '\u0168',
		'n': '\u00f1', 'N': '\u00d1', ' ': '\u007e',
	},
	0xfe54: {
		'a': '\u0101', 'e': '\u0113', 'i': '\u012b', 'o': '\u014d', 'u': '\u016b',
		'A': '\u0100', 'E': '\u0112', 'I': '\u012a', 'O': '\u014c', 'U': '\u016a',
		'g': '\u1e21', 'G': '\u1e20', ' ': '\u00af',
	},
	0xfe55: {
		'a': '\u0103', 'e': '\u0115', 'i': '\u012d', 'o': '\u014f', 'u': '\u016d',
		'A': '\u0102', 'E': '\u0114', 'I': '\u012c', 'O': '\u014e', 'U': '\u016c',
		'g': '\u011f', 'G': '\u011e', ' ': '\u02d8',
	},
	0xfe56: {
		'c': '\u010b', 'C': '\u010a', 'e': '\u0117', 'E': '\u0116', 'g': '\u0121', 'G': '\u0120',
		'z': '\u017c', 'Z': '\u017b', ' ': '\u02d9',
	},
	0xfe57: {
		'a': '\u00e4', 'e': '\u00eb', 'i': '\u00ef', 'o': '\u00f6', 'u': '\u00fc', 'y': '\u00ff',
		'A': '\u00c4', 'E': '\u00cb', 'I': '\u00cf', 'O': '\u00d6', 'U': '\u00dc', 'Y': '\u0178',
		' ': '\u00a8',
	},
	0xfe58: {
		'a': '\u00e5', 'A': '\u00c5', 'u': '\u016f', 'U': '\u016e', ' ': '\u02da',
	},
	0xfe59: {
		'o': '\u0151', 'u': '\u0171', 'O': '\u0150', 'U': '\u0170', ' ': '\u02dd',
	},
	0xfe5a: {
		'a': '\u01ce', 'A': '\u01cd', 'c': '\u010d', 'C': '\u010c', 'd': '\u010f', 'D': '\u010e',
		'e': '\u011b', 'E': '\u011a', 'g': '\u01e7', 'G': '\u01e6', 'h': '\u021f', 'H': '\u021e',
		'i': '\u01d0', 'I': '\u01cf', 'j': '\u01f0', 'k': '\u01e9', 'K': '\u01e8',
		'l': '\u013e', 'L': '\u013d', 'n': '\u0148', 'N': '\u0147', 'o': '\u01d2', 'O': '\u01d1',
		'r': '\u0159', 'R': '\u0158', 's': '\u0161', 'S': '\u0160', 't': '\u0165', 'T': '\u0164',
		'u': '\u01d4', 'U': '\u01d3', 'z': '\u017e', 'Z': '\u017d', ' ': '\u02c7',
	},
	0xfe5b: {
		'c': '\u00e7', 'C': '\u00c7', 's': '\u015f', 'S': '\u015e', 't': '\u0163', 'T': '\u0162',
		'n': '\u0146', 'N': '\u0145', 'r': '\u0157', 'R': '\u0156', 'l': '\u013c', 'L': '\u013b',
		'g': '\u0123', 'G': '\u0122', 'k': '\u0137', 'K': '\u0136', ' ': '\u00b8',
	},
	0xfe5c: {
		'a': '\u0105', 'e': '\u0119', 'i': '\u012f', 'o': '\u01eb', 'u': '\u0173',
		'A': '\u0104', 'E': '\u0118', 'I': '\u012e', 'O': '\u01ea', 'U': '\u0172',
		' ': '\u02db',
	},
	0xfe60: {
		'a': '\u1ea1', 'e': '\u1eb9', 'i': '\u1ecb', 'o': '\u1ecd', 'u': '\u1ee5', 'y': '\u1ef5',
		'A': '\u1ea0', 'E': '\u1eb8', 'I': '\u1eca', 'O': '\u1ecc', 'U': '\u1ee4', 'Y': '\u1ef4',
		' ': '\u002e',
	},
	0xfe61: {
		'a': '\u1ea3', 'e': '\u1ebb', 'i': '\u1ec9', 'o': '\u1ecf', 'u': '\u1ee7', 'y': '\u1ef7',
		'A': '\u1ea2', 'E': '\u1eba', 'I': '\u1ec8', 'O': '\u1ece', 'U': '\u1ee6', 'Y': '\u1ef6',
		' ': '\u02bc',
	},
	0xfe62: {
		'o': '\u01a1', 'u': '\u01b0', 'O': '\u01a0', 'U': '\u01af', ' ': '\u02bc',
	},
	0xfe63: {
		'o': '\u00f8', 'O': '\u00d8', 'l': '\u0142', 'L': '\u0141', 'd': '\u0111', 'D': '\u0110',
		'h': '\u0127', 'H': '\u0126', 't': '\u0167', 'T': '\u0166', ' ': '\u002f',
	},
	0xfe64: {
		' ': '\u02bc',
	},
	0xfe65: {
		' ': '\u02bd',
	},
	0xfe66: {
		'a': '\u0201', 'e': '\u0205', 'i': '\u0209', 'o': '\u020d', 'u': '\u0215',
		'A': '\u0200', 'E': '\u0204', 'I': '\u0208', 'O': '\u020c', 'U': '\u0214',
		'r': '\u0211', 'R': '\u0210', ' ': '\u02f5',
	},
	0xfe6e: {
		's': '\u0219', 'S': '\u0218', 't': '\u021b', 'T': '\u021a', ' ': '\u002c',
	},
}

func composeDead(dead xproto.Keysym, base rune) (rune, bool) {
	if m, ok := deadCompose[dead]; ok {
		if r, ok := m[base]; ok {
			return r, true
		}
	}
	return 0, false
}

func (kh *KeyHandler) Translate(ev xproto.KeyPressEvent, flags ...bool) ([]byte, ActionType) {
	keysym := kh.KeySym(ev)
	if keysym == 0 {
		return nil, ActionNone
	}

	isAppCursor := len(flags) > 0 && flags[0]
	isAltScreen := len(flags) > 1 && flags[1]
	state := ev.State
	isShift := (state & ModShift) != 0
	isCtrl := (state & ModCtrl) != 0
	isAlt := (state & ModAlt) != 0

	if keysym == 0xfe20 {
		if isCtrl && isShift {
			return nil, ActionPrevTab
		}
		if isAlt {
			return []byte("\x1b\x1b[Z"), ActionNone
		}
		return []byte("\x1b[Z"), ActionNone
	}

	if isDeadKeysym(keysym) {
		if kh.pendingDead != 0 {
			prev := kh.pendingDead
			kh.pendingDead = keysym
			return encodeRune(deadSpacing(prev)), ActionNone
		}
		kh.pendingDead = keysym
		return nil, ActionNone
	}
	if kh.pendingDead != 0 {
		if r, ok := KeysymToRune(keysym); ok && !isCtrl && !isAlt &&
			(keysym < 0xff00 || (keysym&0xff000000) == 0x01000000) {
			dead := kh.pendingDead
			kh.pendingDead = 0
			if composed, ok := composeDead(dead, r); ok {
				return encodeRune(composed), ActionNone
			}

			return append(encodeRune(deadSpacing(dead)), encodeRune(r)...), ActionNone
		}

		kh.pendingDead = 0
	}

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

	if seq := funcKeySeq(keysym, isShift, isAlt, isCtrl); seq != nil {
		return seq, ActionNone
	}

	var seq []byte
	switch keysym {
	case 0xffb0, 0xffb1, 0xffb2, 0xffb3, 0xffb4, 0xffb5, 0xffb6, 0xffb7, 0xffb8, 0xffb9:
		seq = []byte{byte('0' + (keysym - 0xffb0))}

	case 0xffaa:
		seq = []byte("*")
	case 0xffab:
		seq = []byte("+")
	case 0xffac:
		seq = []byte(",")
	case 0xffad:
		seq = []byte("-")
	case 0xffae:
		seq = []byte(".")
	case 0xffaf:
		seq = []byte("/")
	case 0xffbd:
		seq = []byte("=")
	case 0xff80:
		seq = []byte(" ")
	case 0xff89:
		seq = []byte("\t")

	case 0xff95:
		if isShift {
			seq = []byte("\x1b[1;2H")
		} else if isAppCursor {
			seq = []byte("\x1bOH")
		} else {
			seq = []byte("\x1b[H")
		}
	case 0xff96:
		if isAppCursor {
			seq = []byte("\x1bOD")
		} else {
			seq = []byte("\x1b[D")
		}
	case 0xff97:
		if isAppCursor {
			seq = []byte("\x1bOA")
		} else {
			seq = []byte("\x1b[A")
		}
	case 0xff98:
		if isAppCursor {
			seq = []byte("\x1bOC")
		} else {
			seq = []byte("\x1b[C")
		}
	case 0xff99:
		if isAppCursor {
			seq = []byte("\x1bOB")
		} else {
			seq = []byte("\x1b[B")
		}
	case 0xff9a:
		if isShift {
			seq = []byte("\x1b[5;2~")
		} else {
			seq = []byte("\x1b[5~")
		}
	case 0xff9b:
		if isShift {
			seq = []byte("\x1b[6;2~")
		} else {
			seq = []byte("\x1b[6~")
		}
	case 0xff9c:
		if isShift {
			seq = []byte("\x1b[1;2F")
		} else if isAppCursor {
			seq = []byte("\x1bOF")
		} else {
			seq = []byte("\x1b[F")
		}
	case 0xff9d:
		seq = []byte("\x1b[E")
	case 0xff9e:
		seq = []byte("\x1b[2~")
	case 0xff9f:
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
			seq = []byte("\x1b[1;2H")
		} else if isAppCursor {
			seq = []byte("\x1bOH")
		} else {
			seq = []byte("\x1b[H")
		}
	case 0xff57:
		if isShift {
			seq = []byte("\x1b[1;2F")
		} else if isAppCursor {
			seq = []byte("\x1bOF")
		} else {
			seq = []byte("\x1b[F")
		}
	case 0xff55:
		if isShift {
			seq = []byte("\x1b[5;2~")
		} else {
			seq = []byte("\x1b[5~")
		}
	case 0xff56:
		if isShift {
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
	}

	if seq != nil {
		if isAlt {
			return append([]byte{0x1b}, seq...), ActionNone
		}
		return seq, ActionNone
	}

	if r, ok := KeysymToRune(keysym); ok {
		buf := make([]byte, 4)
		n := utf8.EncodeRune(buf, r)
		if isAlt {
			return append([]byte{0x1b}, buf[:n]...), ActionNone
		}
		return buf[:n], ActionNone
	}

	return nil, ActionNone
}

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

func KeysymToRune(keysym xproto.Keysym) (rune, bool) {
	if keysym >= 0xffb0 && keysym <= 0xffb9 {
		return rune('0' + (keysym - 0xffb0)), true
	}

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

	if keysym >= 0x0020 && keysym <= 0x007e {
		return rune(keysym), true
	}

	if keysym >= 0x00a0 && keysym <= 0x00ff {
		return rune(keysym), true
	}

	if (keysym & 0xff000000) == 0x01000000 {
		cp := rune(keysym & 0x00ffffff)
		if cp >= 0x20 && cp <= 0x10ffff {
			return cp, true
		}
	}

	if keysym >= 0xfd00 && keysym <= 0xfeff {
		return 0, false
	}

	if r, ok := legacyGreekCyrillicKeysyms[keysym]; ok {
		return r, true
	}
	if (keysym >= 0x06a0 && keysym <= 0x06ff) || (keysym >= 0x07a1 && keysym <= 0x07fe) {
		return 0, false
	}

	if keysym >= 0x0100 && keysym <= 0x10ffff && (keysym < 0xff00 || keysym > 0xffff) {
		return rune(keysym), true
	}

	return 0, false
}
