package gotogen

import (
	"tinygo.org/x/drivers"
)

type Display interface {
	drivers.Displayer

	// CanUpdateNow indicates that the device is able to take a new frame right now. This is useful if the device is
	// driven by a DMA transfer, and the previous transfer has not yet completed. Displays should still support Display
	// being called before they are ready to update; in this case, they should block until the next update is possible.
	CanUpdateNow() bool
}

type MenuButton uint8

const (
	MenuButtonNone MenuButton = iota
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
	statusStateBoot statusState = iota
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

type faceState uint8

const (
	faceStateBusy faceState = iota
	faceStateDefault
	faceStateAnimation // TODO maybe each animation type is defined here to make it easier?
)

func (s faceState) String() string {
	switch s {
	case faceStateBusy:
		return "busy"
	case faceStateDefault:
		return "default"
	case faceStateAnimation:
		return "animation"
	default:
		return "INVALID"
	}
}

type colorChannel uint8

const (
	colorChannelNone colorChannel = iota
	colorChannelRed
	colorChannelGreen
	colorChannelBlue
)
