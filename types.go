package gotogen

type MenuButton uint8

const (
	MenuButtonNone = iota
	MenuButtonMenu
	MenuButtonBack
	MenuButtonUp
	MenuButtonDown
	// MenuButtonDefault is for resetting a specific setting to its default value. Drivers may wish to require this
	// button to be held down for a second before triggering it, or perhaps make it be a chord of up and down.
	MenuButtonDefault
)

func (b MenuButton) String() string {
	switch b {
	case MenuButtonNone:
		return "none"
	case MenuButtonMenu:
		return "menu"
	case MenuButtonBack:
		return "back"
	case MenuButtonUp:
		return "up"
	case MenuButtonDown:
		return "down"
	case MenuButtonDefault:
		return "default"
	default:
		return "INVALID"
	}
}

// statusState indicates what mode the status screen is in.
type statusState uint8

const (
	statusStateBoot = iota
	statusStateIdle
	statusStateMenu
	statusStateBlank
)

func (s statusState) String() string {
	switch s {
	case statusStateBoot:
		return "boot"
	case statusStateIdle:
		return "idle"
	case statusStateMenu:
		return "menu"
	case statusStateBlank:
		return "blank"
	default:
		return "INVALID"
	}
}
