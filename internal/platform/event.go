package platform

type Event interface {
	isPlatformEvent()
}

type MotionNotifyEvent struct {
	EventX int16
	EventY int16
	RootX  int16
	RootY  int16
	State  uint16
}

func (MotionNotifyEvent) isPlatformEvent() {}

type ButtonPressEvent struct {
	Detail uint8
	State  uint16
	EventX int16
	EventY int16
	RootX  int16
	RootY  int16
}

func (ButtonPressEvent) isPlatformEvent() {}

type ButtonReleaseEvent struct {
	Detail uint8
	State  uint16
	EventX int16
	EventY int16
	RootX  int16
	RootY  int16
}

func (ButtonReleaseEvent) isPlatformEvent() {}

type KeyPressEvent struct {
	Detail uint8
	State  uint16
	KeySym uint32
	Bytes  []byte
	Action ActionType
}

func (KeyPressEvent) isPlatformEvent() {}

type ConfigureNotifyEvent struct {
	Width  uint16
	Height uint16
}

func (ConfigureNotifyEvent) isPlatformEvent() {}

type ExposeEvent struct{}

func (ExposeEvent) isPlatformEvent() {}

type FocusInEvent struct{}

func (FocusInEvent) isPlatformEvent() {}

type FocusOutEvent struct{}

func (FocusOutEvent) isPlatformEvent() {}

type PasteNotifyEvent struct {
	Text string
}

func (PasteNotifyEvent) isPlatformEvent() {}

type CloseRequestEvent struct{}

func (CloseRequestEvent) isPlatformEvent() {}

type MappingNotifyEvent struct{}

func (MappingNotifyEvent) isPlatformEvent() {}
