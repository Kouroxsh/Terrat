//go:build linux

package platform

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/bigreq"
	"github.com/jezek/xgb/xproto"
)

type Window struct {
	X      *xgb.Conn
	Win    xproto.Window
	GC     xproto.Gcontext
	Screen *xproto.ScreenInfo
	Depth  byte

	Width  uint16
	Height uint16

	AtomWmProtocols     xproto.Atom
	AtomWmDeleteWindow  xproto.Atom
	AtomNetWmName       xproto.Atom
	AtomUtf8String      xproto.Atom
	AtomNetWmMoveresize xproto.Atom
	AtomNetWmState      xproto.Atom
	AtomNetWmMaxV       xproto.Atom
	AtomNetWmMaxH       xproto.Atom
	AtomChangeState     xproto.Atom
	AtomClipboard       xproto.Atom
	AtomTargets         xproto.Atom
	AtomTerratPaste     xproto.Atom
	AtomNetWmOpacity    xproto.Atom
	AtomText            xproto.Atom
	AtomCompoundText    xproto.Atom
	AtomIncr            xproto.Atom
	AtomMultiple        xproto.Atom
	AtomTimestamp       xproto.Atom

	clipMu        sync.RWMutex
	clipboardText string
	primaryText   string

	clipboardTime xproto.Timestamp
	primaryTime   xproto.Timestamp

	lastEventTime atomic.Uint32

	incrMu sync.Mutex

	incrOut map[incrTransferKey]*incrOutState

	incrIn *incrInState

	pasteMu sync.Mutex

	pastePending *pasteRequest

	pixmap        xproto.Pixmap
	pixmapW       uint16
	pixmapH       uint16
	cursors       map[CursorType]xproto.Cursor
	currentCursor CursorType

	keyHandler *KeyHandler

	AppCursorFunc func() bool

	AltScreenFunc func() bool
	eventCh       chan Event

	done chan struct{}

	loopDone chan struct{}

	closeOnce sync.Once

	closeEventsOnce sync.Once
}

func NewWindow(title string, width, height uint16, iconPath string, iconData []byte) (*Window, error) {
	X, err := xgb.NewConn()
	if err != nil {
		displayEnv := os.Getenv("DISPLAY")
		waylandEnv := os.Getenv("WAYLAND_DISPLAY")
		if waylandEnv != "" && displayEnv == "" {
			return nil, fmt.Errorf("cannot connect to display server: running Wayland (%s) but DISPLAY is not set (Xwayland is required). Please ensure xorg-xwayland is installed: %w", waylandEnv, err)
		}
		return nil, fmt.Errorf("cannot connect to X11 display (DISPLAY=%q): %w", displayEnv, err)
	}

	if err := bigreq.Init(X); err == nil {
		_, _ = bigreq.Enable(X).Reply()
	}

	setup := xproto.Setup(X)
	screen := setup.DefaultScreen(X)

	winId, err := xproto.NewWindowId(X)
	if err != nil {
		X.Close()
		return nil, fmt.Errorf("failed to allocate window id: %w", err)
	}

	eventMask := uint32(
		xproto.EventMaskExposure |
			xproto.EventMaskKeyPress |
			xproto.EventMaskStructureNotify |
			xproto.EventMaskFocusChange |
			xproto.EventMaskButtonPress |
			xproto.EventMaskButtonRelease |
			xproto.EventMaskPointerMotion |
			xproto.EventMaskButtonMotion |
			xproto.EventMaskPropertyChange |

			xproto.EventMaskKeymapState,
	)

	createMask := uint32(
		xproto.CwBackPixmap |
			xproto.CwBitGravity |
			xproto.CwWinGravity |
			xproto.CwEventMask,
	)
	createVals := []uint32{
		xproto.BackPixmapNone,
		xproto.GravityNorthWest,
		xproto.GravityNorthWest,
		eventMask,
	}

	err = xproto.CreateWindowChecked(
		X,
		screen.RootDepth,
		winId,
		screen.Root,
		80, 80,
		width, height,
		0,
		xproto.WindowClassInputOutput,
		screen.RootVisual,
		createMask,
		createVals,
	).Check()
	if err != nil {
		X.Close()
		return nil, fmt.Errorf("failed to create X11 window: %w", err)
	}

	gcId, err := xproto.NewGcontextId(X)
	if err != nil {
		X.Close()
		return nil, fmt.Errorf("failed to allocate GC id: %w", err)
	}

	err = xproto.CreateGCChecked(X, gcId, xproto.Drawable(winId), xproto.GcGraphicsExposures, []uint32{0}).Check()
	if err != nil {
		X.Close()
		return nil, fmt.Errorf("failed to create GC: %w", err)
	}

	w := &Window{
		X:             X,
		Win:           winId,
		GC:            gcId,
		Screen:        screen,
		Depth:         screen.RootDepth,
		Width:         width,
		Height:        height,
		cursors:       make(map[CursorType]xproto.Cursor),
		currentCursor: CursorDefault,
		done:          make(chan struct{}),
		loopDone:      make(chan struct{}),
	}

	w.AtomWmProtocols, _ = internAtom(X, "WM_PROTOCOLS")
	w.AtomWmDeleteWindow, _ = internAtom(X, "WM_DELETE_WINDOW")
	w.AtomNetWmName, _ = internAtom(X, "_NET_WM_NAME")
	w.AtomUtf8String, _ = internAtom(X, "UTF8_STRING")
	w.AtomNetWmMoveresize, _ = internAtom(X, "_NET_WM_MOVERESIZE")
	w.AtomNetWmState, _ = internAtom(X, "_NET_WM_STATE")
	w.AtomNetWmMaxV, _ = internAtom(X, "_NET_WM_STATE_MAXIMIZED_VERT")
	w.AtomNetWmMaxH, _ = internAtom(X, "_NET_WM_STATE_MAXIMIZED_HORZ")
	w.AtomChangeState, _ = internAtom(X, "WM_CHANGE_STATE")
	w.AtomClipboard, _ = internAtom(X, "CLIPBOARD")
	w.AtomTargets, _ = internAtom(X, "TARGETS")
	w.AtomTerratPaste, _ = internAtom(X, "TERRAT_SELECTION")
	w.AtomNetWmOpacity, _ = internAtom(X, "_NET_WM_WINDOW_OPACITY")
	w.AtomText, _ = internAtom(X, "TEXT")
	w.AtomCompoundText, _ = internAtom(X, "COMPOUND_TEXT")
	w.AtomIncr, _ = internAtom(X, "INCR")
	w.AtomMultiple, _ = internAtom(X, "MULTIPLE")
	w.AtomTimestamp, _ = internAtom(X, "TIMESTAMP")

	motifAtom, err := internAtom(X, "_MOTIF_WM_HINTS")
	if err == nil && motifAtom != 0 {
		hints := []uint32{2, 0, 0, 0, 0}
		data := make([]byte, 20)
		for i, val := range hints {
			binary.LittleEndian.PutUint32(data[i*4:], val)
		}
		_ = xproto.ChangeProperty(
			X,
			xproto.PropModeReplace,
			winId,
			motifAtom,
			motifAtom,
			32,
			5,
			data,
		)
	}

	if w.AtomWmProtocols != 0 && w.AtomWmDeleteWindow != 0 {
		atomBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(atomBytes, uint32(w.AtomWmDeleteWindow))
		_ = xproto.ChangePropertyChecked(
			X,
			xproto.PropModeReplace,
			winId,
			w.AtomWmProtocols,
			xproto.AtomAtom,
			32,
			1,
			atomBytes,
		).Check()
	}

	wmClassAtom, err := internAtom(X, "WM_CLASS")
	if err == nil && wmClassAtom != 0 {
		stringAtom, _ := internAtom(X, "STRING")
		if stringAtom == 0 {
			stringAtom = xproto.AtomString
		}
		classData := []byte("terrat\x00TerraTerminal\x00")
		_ = xproto.ChangePropertyChecked(
			X,
			xproto.PropModeReplace,
			winId,
			wmClassAtom,
			stringAtom,
			8,
			uint32(len(classData)),
			classData,
		).Check()
	}

	pidAtom, err := internAtom(X, "_NET_WM_PID")
	cardinalAtom, _ := internAtom(X, "CARDINAL")
	if err == nil && pidAtom != 0 && cardinalAtom != 0 {
		pidBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(pidBytes, uint32(os.Getpid()))
		_ = xproto.ChangePropertyChecked(
			X,
			xproto.PropModeReplace,
			winId,
			pidAtom,
			cardinalAtom,
			32,
			1,
			pidBytes,
		).Check()
	}

	winTypeAtom, err := internAtom(X, "_NET_WM_WINDOW_TYPE")
	normalTypeAtom, err2 := internAtom(X, "_NET_WM_WINDOW_TYPE_NORMAL")
	atomAtom, _ := internAtom(X, "ATOM")
	if err == nil && err2 == nil && winTypeAtom != 0 && normalTypeAtom != 0 {
		typeBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(typeBytes, uint32(normalTypeAtom))
		_ = xproto.ChangePropertyChecked(
			X,
			xproto.PropModeReplace,
			winId,
			winTypeAtom,
			atomAtom,
			32,
			1,
			typeBytes,
		).Check()
	}

	cursorFontId, err := xproto.NewFontId(X)
	if err != nil {
		fmt.Fprintf(os.Stderr, "terrat: failed to allocate cursor font id: %v\n", err)
	} else if err := xproto.OpenFontChecked(X, cursorFontId, uint16(len("cursor")), "cursor").Check(); err != nil {
		fmt.Fprintf(os.Stderr, "terrat: failed to open cursor font: %v\n", err)
	} else {
		glyphMap := map[CursorType]uint16{
			CursorDefault:           68,
			CursorText:              152,
			CursorPointer:           58,
			CursorResizeTopLeft:     134,
			CursorResizeTop:         138,
			CursorResizeTopRight:    136,
			CursorResizeRight:       96,
			CursorResizeBottomRight: 14,
			CursorResizeBottom:      16,
			CursorResizeBottomLeft:  12,
			CursorResizeLeft:        70,
		}
		for ct, glyph := range glyphMap {
			cid, err := xproto.NewCursorId(X)
			if err != nil {
				fmt.Fprintf(os.Stderr, "terrat: failed to allocate cursor id: %v\n", err)
				continue
			}
			if err := xproto.CreateGlyphCursorChecked(X, cid, cursorFontId, cursorFontId,
				glyph, glyph+1, 0, 0, 0, 65535, 65535, 65535).Check(); err != nil {
				fmt.Fprintf(os.Stderr, "terrat: failed to create cursor %d: %v\n", ct, err)
				continue
			}
			w.cursors[ct] = cid
		}
	}

	w.SetTitle(title)

	if len(iconData) > 0 {
		if err := SetWindowIconBytes(X, winId, iconData); err != nil {
			fmt.Fprintf(os.Stderr, "terrat: failed to set window icon: %v\n", err)
		}
	} else if iconPath != "" {
		if err := SetWindowIcon(X, winId, iconPath); err != nil {
			fmt.Fprintf(os.Stderr, "terrat: failed to set window icon: %v\n", err)
		}
	}

	w.SetMinSize(320, 180)

	_ = xproto.MapWindowChecked(X, winId).Check()
	X.Sync()

	kh, err := NewKeyHandler(X)
	if err != nil {
		X.Close()
		return nil, fmt.Errorf("initializing keyboard handler: %w", err)
	}
	w.keyHandler = kh
	w.eventCh = make(chan Event, 1024)
	go w.eventLoop()

	return w, nil
}

func (w *Window) SetTitle(title string) {
	if w.AtomNetWmName != 0 && w.AtomUtf8String != 0 {
		_ = xproto.ChangeProperty(
			w.X,
			xproto.PropModeReplace,
			w.Win,
			w.AtomNetWmName,
			w.AtomUtf8String,
			8,
			uint32(len(title)),
			[]byte(title),
		)
	}

	latin1 := toLatin1(title)
	_ = xproto.ChangeProperty(
		w.X,
		xproto.PropModeReplace,
		w.Win,
		xproto.AtomWmName,
		xproto.AtomString,
		8,
		uint32(len(latin1)),
		latin1,
	)
}

func toLatin1(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xFF || (r >= 0x80 && r < 0xA0) {
			out = append(out, '?')
		} else {
			out = append(out, byte(r))
		}
	}
	return out
}

func (w *Window) SetCursorType(ct CursorType) {
	if ct == w.currentCursor {
		return
	}
	cid, ok := w.cursors[ct]
	if !ok {
		return
	}
	w.currentCursor = ct
	_ = xproto.ChangeWindowAttributes(w.X, w.Win, xproto.CwCursor, []uint32{uint32(cid)})
}

func (w *Window) SetOpacity(opacity float64) {
	if w.AtomNetWmOpacity == 0 {
		return
	}
	if opacity < 0.1 {
		opacity = 0.1
	}
	if opacity > 1.0 {
		opacity = 1.0
	}
	val := uint32(opacity * 4294967295.0)
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, val)

	_ = xproto.ChangePropertyChecked(
		w.X,
		xproto.PropModeReplace,
		w.Win,
		w.AtomNetWmOpacity,
		xproto.AtomCardinal,
		32,
		1,
		buf,
	).Check()
}

func (w *Window) StartResize(direction int, rootX, rootY int16) {
	if w.AtomNetWmMoveresize == 0 || direction < 0 || direction > 8 {
		return
	}

	if rootX < 0 {
		rootX = 0
	}
	if rootY < 0 {
		rootY = 0
	}
	cm := xproto.ClientMessageEvent{
		Format: 32,
		Window: w.Win,
		Type:   w.AtomNetWmMoveresize,
		Data: xproto.ClientMessageDataUnionData32New([]uint32{
			uint32(rootX),
			uint32(rootY),
			uint32(direction),
			1,
			1,
		}),
	}
	_ = xproto.SendEvent(
		w.X,
		false,
		w.Screen.Root,
		xproto.EventMaskSubstructureNotify|xproto.EventMaskSubstructureRedirect,
		string(cm.Bytes()),
	)
	w.X.Sync()
}

func (w *Window) StartDrag(rootX, rootY int16) {
	w.StartResize(ResizeMove, rootX, rootY)
}

func (w *Window) Minimize() {
	if w.AtomChangeState == 0 {
		return
	}
	cm := xproto.ClientMessageEvent{
		Format: 32,
		Window: w.Win,
		Type:   w.AtomChangeState,
		Data: xproto.ClientMessageDataUnionData32New([]uint32{
			3,
			0, 0, 0, 0,
		}),
	}
	_ = xproto.SendEvent(
		w.X,
		false,
		w.Screen.Root,
		xproto.EventMaskSubstructureNotify|xproto.EventMaskSubstructureRedirect,
		string(cm.Bytes()),
	)
	w.X.Sync()
}

func (w *Window) ToggleMaximize() {
	if w.AtomNetWmState == 0 || w.AtomNetWmMaxV == 0 || w.AtomNetWmMaxH == 0 {
		return
	}
	cm := xproto.ClientMessageEvent{
		Format: 32,
		Window: w.Win,
		Type:   w.AtomNetWmState,
		Data: xproto.ClientMessageDataUnionData32New([]uint32{
			2,
			uint32(w.AtomNetWmMaxV),
			uint32(w.AtomNetWmMaxH),
			1,
			0,
		}),
	}
	_ = xproto.SendEvent(
		w.X,
		false,
		w.Screen.Root,
		xproto.EventMaskSubstructureNotify|xproto.EventMaskSubstructureRedirect,
		string(cm.Bytes()),
	)
	w.X.Sync()
}

func (w *Window) SetMinSize(minW, minH uint32) {
	atomNormalHints, _ := internAtom(w.X, "WM_NORMAL_HINTS")
	atomSizeHints, _ := internAtom(w.X, "WM_SIZE_HINTS")
	if atomNormalHints == 0 || atomSizeHints == 0 {
		return
	}
	hints := make([]uint32, 18)
	hints[0] = 0x10
	hints[5] = minW
	hints[6] = minH

	data := make([]byte, 18*4)
	for i, v := range hints {
		binary.LittleEndian.PutUint32(data[i*4:], v)
	}
	_ = xproto.ChangeProperty(
		w.X,
		xproto.PropModeReplace,
		w.Win,
		atomNormalHints,
		atomSizeHints,
		32,
		18,
		data,
	)
}

const maxPutImageBytes = 65536 - 32

func (w *Window) Blit(pixels []byte, width, height uint16) {
	if len(pixels) == 0 || width == 0 || height == 0 {
		return
	}

	if w.pixmap == 0 || w.pixmapW != width || w.pixmapH != height {
		if w.pixmap != 0 {
			_ = xproto.FreePixmap(w.X, w.pixmap)
			w.pixmap = 0
		}
		pid, err := xproto.NewPixmapId(w.X)
		if err == nil {
			err = xproto.CreatePixmapChecked(w.X, w.Depth, pid, xproto.Drawable(w.Win), width, height).Check()
			if err == nil {
				w.pixmap = pid
				w.pixmapW = width
				w.pixmapH = height
			}
		}
	}

	targetDrawable := xproto.Drawable(w.Win)
	if w.pixmap != 0 {
		targetDrawable = xproto.Drawable(w.pixmap)
	}

	stride := int(width) * 4
	h := int(height)
	if stride > maxPutImageBytes {
		maxCols := maxPutImageBytes / 4
		for y := 0; y < h; y++ {
			for x0 := 0; x0 < int(width); x0 += maxCols {
				chunkW := maxCols
				if x0+chunkW > int(width) {
					chunkW = int(width) - x0
				}
				start := y*stride + x0*4
				end := start + chunkW*4
				if end > len(pixels) {
					end = len(pixels)
				}
				_ = xproto.PutImage(
					w.X,
					xproto.ImageFormatZPixmap,
					targetDrawable,
					w.GC,
					uint16(chunkW),
					1,
					int16(x0), int16(y),
					0,
					w.Depth,
					pixels[start:end],
				)
			}
		}
	} else {
		maxRowsPerChunk := maxPutImageBytes / stride
		if maxRowsPerChunk < 1 {
			maxRowsPerChunk = 1
		}

		for y := 0; y < h; y += maxRowsPerChunk {
			chunkH := maxRowsPerChunk
			if y+chunkH > h {
				chunkH = h - y
			}
			start := y * stride
			end := start + (chunkH * stride)
			if end > len(pixels) {
				end = len(pixels)
			}

			_ = xproto.PutImage(
				w.X,
				xproto.ImageFormatZPixmap,
				targetDrawable,
				w.GC,
				width,
				uint16(chunkH),
				0, int16(y),
				0,
				w.Depth,
				pixels[start:end],
			)
		}
	}

	if w.pixmap != 0 {
		_ = xproto.CopyArea(
			w.X,
			xproto.Drawable(w.pixmap),
			xproto.Drawable(w.Win),
			w.GC,
			0, 0,
			0, 0,
			width, height,
		)
	}

	w.X.Sync()
}

func (w *Window) closeEvents() {
	w.closeEventsOnce.Do(func() { close(w.eventCh) })
}

func (w *Window) sendEvent(ev Event) bool {
	select {
	case w.eventCh <- ev:
		return true
	case <-w.done:
		return false
	}
}

func (w *Window) Close() {
	w.closeOnce.Do(func() {
		close(w.done)
		w.X.Close()
		select {
		case <-w.loopDone:
		case <-time.After(2 * time.Second):
		}

		if w.pixmap != 0 {
			_ = xproto.FreePixmap(w.X, w.pixmap)
			w.pixmap = 0
		}
		_ = xproto.FreeGC(w.X, w.GC)
		_ = xproto.DestroyWindow(w.X, w.Win)
	})
}

const incrChunkSize = 64 * 1024

type incrTransferKey struct {
	requestor xproto.Window
	property  xproto.Atom
}

type incrOutState struct {
	data      []byte
	target    xproto.Atom
	chunkSize int
}

func (st *incrOutState) nextChunk() ([]byte, bool) {
	if len(st.data) == 0 {
		return nil, true
	}
	n := len(st.data)
	if n > st.chunkSize {
		n = st.chunkSize
	}
	chunk := st.data[:n]
	st.data = st.data[n:]
	return chunk, false
}

type incrInState struct {
	property xproto.Atom
	buf      []byte
}

type pasteRequest struct {
	selection xproto.Atom
	targetIdx int
}

func (w *Window) pasteTargets() []xproto.Atom {
	targets := make([]xproto.Atom, 0, 4)
	for _, t := range []xproto.Atom{w.AtomUtf8String, xproto.AtomString, w.AtomText, w.AtomCompoundText} {
		if t != 0 {
			targets = append(targets, t)
		}
	}
	return targets
}

func (w *Window) selectionTime() xproto.Timestamp {
	if t := w.lastEventTime.Load(); t != 0 {
		return xproto.Timestamp(t)
	}
	return xproto.TimeCurrentTime
}

func (w *Window) SetClipboard(text string) {
	ts := w.selectionTime()
	w.clipMu.Lock()
	w.clipboardText = text
	w.clipboardTime = ts
	w.clipMu.Unlock()
	if w.AtomClipboard != 0 {
		owner := w.Win
		if text == "" {
			owner = 0
		}
		_ = xproto.SetSelectionOwner(w.X, owner, w.AtomClipboard, ts)
	}
}

func (w *Window) SetPrimary(text string) {
	ts := w.selectionTime()
	w.clipMu.Lock()
	w.primaryText = text
	w.primaryTime = ts
	w.clipMu.Unlock()
	owner := w.Win
	if text == "" {
		owner = 0
	}
	_ = xproto.SetSelectionOwner(w.X, owner, xproto.AtomPrimary, ts)
}

func eventTime(ev xgb.Event) uint32 {
	switch e := ev.(type) {
	case xproto.KeyPressEvent:
		return uint32(e.Time)
	case xproto.KeyReleaseEvent:
		return uint32(e.Time)
	case xproto.ButtonPressEvent:
		return uint32(e.Time)
	case xproto.ButtonReleaseEvent:
		return uint32(e.Time)
	case xproto.MotionNotifyEvent:
		return uint32(e.Time)
	case xproto.SelectionNotifyEvent:
		return uint32(e.Time)
	case xproto.SelectionRequestEvent:
		return uint32(e.Time)
	case xproto.SelectionClearEvent:
		return uint32(e.Time)
	case xproto.PropertyNotifyEvent:
		return uint32(e.Time)
	}
	return 0
}

func (w *Window) HandleSelectionRequest(e xproto.SelectionRequestEvent) {
	target := e.Target
	property := e.Property
	if property == 0 {
		property = target
	}

	success := false
	if target == w.AtomMultiple && w.AtomMultiple != 0 {
		success = w.handleMultipleRequest(e)
	} else {
		success = w.serveSelectionTarget(e.Requestor, property, e.Selection, target)
	}

	respProperty := property
	if !success {
		respProperty = 0
	}

	resp := xproto.SelectionNotifyEvent{
		Time:      e.Time,
		Requestor: e.Requestor,
		Selection: e.Selection,
		Target:    target,
		Property:  respProperty,
	}

	_ = xproto.SendEvent(
		w.X,
		false,
		e.Requestor,
		0,
		string(resp.Bytes()),
	)
	w.X.Sync()
}

func (w *Window) selectionTargets() []uint32 {
	targets := []uint32{
		uint32(w.AtomTargets),
		uint32(w.AtomUtf8String),
		uint32(xproto.AtomString),
	}
	if w.AtomText != 0 {
		targets = append(targets, uint32(w.AtomText))
	}
	if w.AtomIncr != 0 {
		targets = append(targets, uint32(w.AtomIncr))
	}
	if w.AtomMultiple != 0 {
		targets = append(targets, uint32(w.AtomMultiple))
	}
	if w.AtomTimestamp != 0 {
		targets = append(targets, uint32(w.AtomTimestamp))
	}
	return targets
}

func (w *Window) selectionTimestamp(selection xproto.Atom) xproto.Timestamp {
	w.clipMu.RLock()
	defer w.clipMu.RUnlock()
	if selection == xproto.AtomPrimary {
		if w.primaryTime != 0 {
			return w.primaryTime
		}
	} else if w.clipboardTime != 0 {
		return w.clipboardTime
	}
	return w.selectionTime()
}

func (w *Window) serveSelectionTarget(requestor xproto.Window, property xproto.Atom, selection xproto.Atom, target xproto.Atom) bool {
	var text string
	w.clipMu.RLock()
	switch selection {
	case xproto.AtomPrimary:
		text = w.primaryText
	case w.AtomClipboard:
		text = w.clipboardText
	}
	w.clipMu.RUnlock()

	if target == w.AtomTargets && w.AtomTargets != 0 {
		targets := w.selectionTargets()
		data := make([]byte, len(targets)*4)
		for i, t := range targets {
			binary.LittleEndian.PutUint32(data[i*4:], t)
		}
		_ = xproto.ChangeProperty(
			w.X,
			xproto.PropModeReplace,
			requestor,
			property,
			xproto.AtomAtom,
			32,
			uint32(len(targets)),
			data,
		)
		return true
	}

	if target == w.AtomTimestamp && w.AtomTimestamp != 0 {
		ts := w.selectionTimestamp(selection)
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, uint32(ts))
		_ = xproto.ChangeProperty(
			w.X,
			xproto.PropModeReplace,
			requestor,
			property,
			w.AtomTimestamp,
			32,
			1,
			data,
		)
		return true
	}

	if target == w.AtomUtf8String || target == xproto.AtomString || (w.AtomText != 0 && target == w.AtomText) {
		data := []byte(text)
		maxLen := int(xproto.Setup(w.X).MaximumRequestLength)*4 - 32
		if maxLen <= 0 || maxLen > 262112 {
			maxLen = 262112
		}
		if len(data) > maxLen && w.AtomIncr != 0 {
			size := make([]byte, 4)
			binary.LittleEndian.PutUint32(size, uint32(len(data)))
			_ = xproto.ChangeProperty(
				w.X,
				xproto.PropModeReplace,
				requestor,
				property,
				w.AtomIncr,
				32,
				1,
				size,
			)

			_ = xproto.ChangeWindowAttributes(
				w.X,
				requestor,
				xproto.CwEventMask,
				[]uint32{xproto.EventMaskPropertyChange},
			)
			chunkSize := incrChunkSize
			if maxLen < chunkSize {
				chunkSize = maxLen
			}
			w.incrMu.Lock()
			if w.incrOut == nil {
				w.incrOut = make(map[incrTransferKey]*incrOutState)
			}
			w.incrOut[incrTransferKey{requestor: requestor, property: property}] = &incrOutState{data: data, target: target, chunkSize: chunkSize}
			w.incrMu.Unlock()
			return true
		}

		if len(data) > maxLen {
			data = data[:maxLen]
		}
		_ = xproto.ChangeProperty(
			w.X,
			xproto.PropModeReplace,
			requestor,
			property,
			target,
			8,
			uint32(len(data)),
			data,
		)
		return true
	}
	return false
}

func (w *Window) handleMultipleRequest(e xproto.SelectionRequestEvent) bool {
	reply, err := xproto.GetProperty(w.X, false, e.Requestor, e.Property, xproto.AtomAtom, 0, 1<<20).Reply()
	if err != nil || reply == nil || reply.Format != 32 {
		return false
	}
	pairs := parseMultiplePairs(reply.Value)
	if pairs == nil {
		return false
	}
	for i, p := range pairs {
		if p[1] == 0 {
			continue
		}
		if p[0] == w.AtomMultiple {
			pairs[i][1] = 0
			continue
		}
		if !w.serveSelectionTarget(e.Requestor, p[1], e.Selection, p[0]) {
			pairs[i][1] = 0
		}
	}
	data := make([]byte, len(pairs)*8)
	for i, p := range pairs {
		binary.LittleEndian.PutUint32(data[i*8:], uint32(p[0]))
		binary.LittleEndian.PutUint32(data[i*8+4:], uint32(p[1]))
	}
	_ = xproto.ChangeProperty(w.X, xproto.PropModeReplace, e.Requestor, e.Property, xproto.AtomAtom, 32, uint32(len(pairs)*2), data)
	return true
}

func parseMultiplePairs(value []byte) [][2]xproto.Atom {
	n := len(value) / 8
	if n == 0 {
		return nil
	}
	pairs := make([][2]xproto.Atom, n)
	for i := range pairs {
		pairs[i][0] = xproto.Atom(binary.LittleEndian.Uint32(value[i*8:]))
		pairs[i][1] = xproto.Atom(binary.LittleEndian.Uint32(value[i*8+4:]))
	}
	return pairs
}

func (w *Window) RequestPaste(selectionAtom xproto.Atom) {
	if selectionAtom == 0 || w.AtomTerratPaste == 0 {
		return
	}
	targets := w.pasteTargets()
	if len(targets) == 0 {
		return
	}
	w.pasteMu.Lock()
	w.pastePending = &pasteRequest{selection: selectionAtom, targetIdx: 0}
	w.pasteMu.Unlock()
	w.requestPasteTarget(selectionAtom, targets[0])
}

func (w *Window) requestPasteTarget(selectionAtom, target xproto.Atom) {
	_ = xproto.ConvertSelection(
		w.X,
		w.Win,
		selectionAtom,
		target,
		w.AtomTerratPaste,
		xproto.TimeCurrentTime,
	)
	w.X.Sync()
}

func (w *Window) retryPaste(e xproto.SelectionNotifyEvent) bool {
	w.pasteMu.Lock()
	pending := w.pastePending
	if pending == nil || e.Selection != pending.selection {
		w.pasteMu.Unlock()
		return false
	}
	targets := w.pasteTargets()

	idx := pending.targetIdx
	for i, t := range targets {
		if t == e.Target {
			idx = i
			break
		}
	}
	if idx+1 >= len(targets) {
		w.pastePending = nil
		w.pasteMu.Unlock()
		return false
	}
	next := targets[idx+1]
	pending.targetIdx = idx + 1
	w.pasteMu.Unlock()
	w.requestPasteTarget(pending.selection, next)
	return true
}

func (w *Window) HandleSelectionNotify(e xproto.SelectionNotifyEvent) []byte {
	if e.Property == 0 {
		w.retryPaste(e)
		return nil
	}

	w.pasteMu.Lock()
	if w.pastePending != nil && e.Selection == w.pastePending.selection {
		w.pastePending = nil
	}
	w.pasteMu.Unlock()

	reply, err := xproto.GetProperty(
		w.X,
		true,
		w.Win,
		e.Property,
		xproto.AtomAny,
		0,
		1024*1024/4,
	).Reply()

	if err != nil || reply == nil || len(reply.Value) == 0 {
		return nil
	}

	if w.AtomIncr != 0 && reply.Type == w.AtomIncr {
		total := uint32(0)
		if len(reply.Value) >= 4 {
			total = binary.LittleEndian.Uint32(reply.Value)
		}
		prealloc := total
		if prealloc > 1024*1024 {
			prealloc = 1024 * 1024
		}
		w.incrMu.Lock()
		w.incrIn = &incrInState{property: e.Property, buf: make([]byte, 0, prealloc)}
		w.incrMu.Unlock()
		return nil
	}

	return reply.Value
}

func (w *Window) handlePropertyNotify(e xproto.PropertyNotifyEvent) []byte {
	if e.State == xproto.PropertyDelete {
		w.incrMu.Lock()
		key := incrTransferKey{requestor: e.Window, property: e.Atom}
		st, ok := w.incrOut[key]
		if ok {
			chunk, done := st.nextChunk()
			target := st.target
			if done {
				delete(w.incrOut, key)
			}
			w.incrMu.Unlock()

			_ = xproto.ChangeProperty(w.X, xproto.PropModeReplace, e.Window, e.Atom, target, 8, uint32(len(chunk)), chunk)
			w.X.Sync()
			return nil
		}
		w.incrMu.Unlock()
	}

	if e.Window == w.Win && e.State == xproto.PropertyNewValue {
		return w.incrReadChunk(e.Atom)
	}
	return nil
}

func (w *Window) incrReadChunk(property xproto.Atom) []byte {
	w.incrMu.Lock()
	st := w.incrIn
	if st == nil || st.property != property {
		w.incrMu.Unlock()
		return nil
	}
	w.incrMu.Unlock()

	reply, err := xproto.GetProperty(w.X, true, w.Win, property, xproto.AtomAny, 0, 1024*1024).Reply()
	if err != nil || reply == nil {
		return nil
	}
	if len(reply.Value) == 0 {
		w.incrMu.Lock()
		w.incrIn = nil
		buf := st.buf
		w.incrMu.Unlock()
		return buf
	}
	w.incrMu.Lock()
	st.buf = append(st.buf, reply.Value...)
	w.incrMu.Unlock()
	return nil
}

func (w *Window) Events() <-chan Event {
	return w.eventCh
}

func (w *Window) Paste() {
	w.RequestPaste(w.AtomClipboard)
}

func (w *Window) PastePrimary() {
	w.RequestPaste(xproto.AtomPrimary)
}

func (w *Window) eventLoop() {
	defer close(w.loopDone)
	consecErrs := 0
	for {
		select {
		case <-w.done:
			w.closeEvents()
			return
		default:
		}
		ev, err := w.X.WaitForEvent()
		if err != nil {
			consecErrs++
			if consecErrs >= 5 {
				w.closeEvents()
				return
			}
			delay := time.Duration(consecErrs) * 10 * time.Millisecond
			select {
			case <-w.done:
				w.closeEvents()
				return
			case <-time.After(delay):
				continue
			}
		}
		consecErrs = 0
		if ev == nil {
			w.closeEvents()
			return
		}

		if t := eventTime(ev); t != 0 {
			w.lastEventTime.Store(t)
		}
		if w.handleEvent(ev) {
			w.closeEvents()
			return
		}
	}
}

func (w *Window) handleEvent(ev xgb.Event) bool {
	switch e := ev.(type) {
	case xproto.MotionNotifyEvent:

		latest, exit := w.coalesce(e, func(ev xgb.Event) bool {
			_, ok := ev.(xproto.MotionNotifyEvent)
			return ok
		})
		if exit {
			return true
		}
		m := latest.(xproto.MotionNotifyEvent)
		if !w.sendEvent(MotionNotifyEvent{
			EventX: m.EventX,
			EventY: m.EventY,
			RootX:  m.RootX,
			RootY:  m.RootY,
			State:  m.State,
		}) {
			return true
		}
	case xproto.ButtonPressEvent:
		if !w.sendEvent(ButtonPressEvent{
			Detail: uint8(e.Detail),
			State:  e.State,
			EventX: e.EventX,
			EventY: e.EventY,
			RootX:  e.RootX,
			RootY:  e.RootY,
		}) {
			return true
		}
	case xproto.ButtonReleaseEvent:
		if !w.sendEvent(ButtonReleaseEvent{
			Detail: uint8(e.Detail),
			State:  e.State,
			EventX: e.EventX,
			EventY: e.EventY,
			RootX:  e.RootX,
			RootY:  e.RootY,
		}) {
			return true
		}
	case xproto.KeyPressEvent:
		var act ActionType
		var b []byte
		var ksym uint32
		if w.keyHandler != nil {
			ksym = uint32(w.keyHandler.KeySym(e))
			appCursor := false
			if w.AppCursorFunc != nil {
				appCursor = w.AppCursorFunc()
			}
			altScreen := false
			if w.AltScreenFunc != nil {
				altScreen = w.AltScreenFunc()
			}
			b, act = w.keyHandler.Translate(e, appCursor, altScreen)
		}
		if !w.sendEvent(KeyPressEvent{
			Detail: byte(e.Detail),
			State:  e.State,
			KeySym: ksym,
			Bytes:  b,
			Action: act,
		}) {
			return true
		}
	case xproto.ConfigureNotifyEvent:

		latest, exit := w.coalesce(e, func(ev xgb.Event) bool {
			_, ok := ev.(xproto.ConfigureNotifyEvent)
			return ok
		})
		if exit {
			return true
		}
		c := latest.(xproto.ConfigureNotifyEvent)
		if !w.sendEvent(ConfigureNotifyEvent{
			Width:  c.Width,
			Height: c.Height,
		}) {
			return true
		}
	case xproto.ExposeEvent:

		if _, exit := w.coalesce(e, func(ev xgb.Event) bool {
			_, ok := ev.(xproto.ExposeEvent)
			return ok
		}); exit {
			return true
		}
		if !w.sendEvent(ExposeEvent{}) {
			return true
		}
	case xproto.FocusInEvent:
		if w.keyHandler != nil {
			w.keyHandler.RefreshXKBState(w.X)
		}

		if !w.sendEvent(FocusInEvent{}) {
			return true
		}
	case xproto.FocusOutEvent:
		if !w.sendEvent(FocusOutEvent{}) {
			return true
		}
	case XKBStateNotifyEvent:
		if len(e.Data) >= 14 && w.keyHandler != nil {
			if e.Data[1] == 2 {
				w.keyHandler.SetGroup(e.Data[13])
			}
			if e.Data[1] == 1 {
				_ = w.keyHandler.RefreshMapping(w.X)
			}
		}
	case xproto.MappingNotifyEvent:
		if w.keyHandler != nil {
			_ = w.keyHandler.RefreshMapping(w.X)
		}
		if !w.sendEvent(MappingNotifyEvent{}) {
			return true
		}
	case xproto.KeymapNotifyEvent:

		if w.keyHandler != nil {
			_ = w.keyHandler.RefreshMapping(w.X)
		}
	case xproto.SelectionRequestEvent:

		go w.HandleSelectionRequest(e)
	case xproto.SelectionNotifyEvent:
		val := w.HandleSelectionNotify(e)
		if len(val) > 0 {
			if !w.sendEvent(PasteNotifyEvent{Text: string(val)}) {
				return true
			}
		}
	case xproto.PropertyNotifyEvent:
		if val := w.handlePropertyNotify(e); len(val) > 0 {
			if !w.sendEvent(PasteNotifyEvent{Text: string(val)}) {
				return true
			}
		}
	case xproto.SelectionClearEvent:

		w.clipMu.Lock()
		if e.Selection == xproto.AtomPrimary {
			w.primaryText = ""
		} else if w.AtomClipboard != 0 && e.Selection == w.AtomClipboard {
			w.clipboardText = ""
		}
		w.clipMu.Unlock()
	case xproto.ClientMessageEvent:

		if e.Type == w.AtomWmProtocols && e.Format == 32 && len(e.Data.Data32) > 0 && xproto.Atom(e.Data.Data32[0]) == w.AtomWmDeleteWindow {
			if !w.sendEvent(CloseRequestEvent{}) {
				return true
			}
		}
	}
	return false
}

func (w *Window) coalesce(first xgb.Event, same func(xgb.Event) bool) (xgb.Event, bool) {
	latest := first
	for i := 0; i < 64; i++ {
		next, err := w.X.PollForEvent()
		if err != nil || next == nil {
			break
		}
		if same(next) {
			latest = next
			continue
		}
		if w.handleEvent(next) {
			return latest, true
		}
	}
	return latest, false
}

func OpenURL(urlStr string) error {
	parsed, err := url.ParseRequestURI(urlStr)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("refusing to open non-http(s) URL %q", urlStr)
	}
	cmd := exec.Command("xdg-open", parsed.String())
	return cmd.Start()
}
