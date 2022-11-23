package gotogen

import (
	"github.com/ajanata/textbuf"
)

type Item interface {
	name() string
}

type Menuable interface {
	Top() uint8
	SetTop(uint8)
	Selected() uint8
	SetSelected(uint8)
	Render(*textbuf.Buffer)
	Len() uint8
	Prev() Menuable
	SetPrev(Menuable)
}

type Menu struct {
	Name     string
	Items    []Item
	selected uint8
	top      uint8
	prev     Menuable
}

func (m *Menu) name() string { return m.Name }

func (m *Menu) Top() uint8 { return m.top }

func (m *Menu) SetTop(t uint8) { m.top = t }

func (m *Menu) Selected() uint8 { return m.selected }

func (m *Menu) SetSelected(s uint8) { m.selected = s }

func (m *Menu) Len() uint8 { return uint8(len(m.Items)) }

func (m *Menu) Prev() Menuable { return m.prev }

func (m *Menu) SetPrev(p Menuable) { m.prev = p }

func (m *Menu) Render(buf *textbuf.Buffer) {
	buf.Clear()
	_, h := buf.Size()
	// TODO center
	buf.SetLineInverse(0, m.Name)
	for i := uint8(0); i+m.top < uint8(len(m.Items)) && i < uint8(h-1); i++ {
		item := m.Items[i+m.top]
		var prefix string
		switch item.(type) {
		case *Menu:
			prefix = "+"
		case ActionItem:
			prefix = "*"
		case *SettingItem:
			prefix = ">"
		}
		if i == m.selected-m.top {
			buf.SetLineInverse(int16(i+1), prefix+item.name())
		} else {
			buf.SetLine(int16(i+1), prefix+item.name())
		}
	}
}

type ActionItem struct {
	Name   string
	Invoke func()
}

func (i ActionItem) name() string {
	return i.Name
}

type SettingItem struct {
	Name     string
	Options  []string
	Default  uint8
	Active   uint8
	top      uint8
	selected uint8
	prev     Menuable
}

func (si *SettingItem) name() string { return si.Name }

func (si *SettingItem) Top() uint8 { return si.top }

func (si *SettingItem) SetTop(t uint8) { si.top = t }

func (si *SettingItem) Selected() uint8 { return si.selected }

func (si *SettingItem) SetSelected(s uint8) { si.selected = s }

func (si *SettingItem) Len() uint8 { return uint8(len(si.Options)) }

func (si *SettingItem) Prev() Menuable { return si.prev }

func (si *SettingItem) SetPrev(p Menuable) { si.prev = p }

func (si *SettingItem) Render(buf *textbuf.Buffer) {
	buf.Clear()
	_, h := buf.Size()
	// TODO center
	buf.SetLineInverse(0, si.Name)
	for i := uint8(0); i+si.top < uint8(len(si.Options)) && i < uint8(h-1); i++ {
		item := si.Options[i+si.top]
		prefix := " "
		if i == si.Active-si.top {
			prefix = "*"
		}
		if i == si.selected-si.top {
			buf.SetLineInverse(int16(i+1), prefix+item)
		} else {
			buf.SetLine(int16(i+1), prefix+item)
		}
	}
}

type MenuProvider interface {
	GetMenu() Menu
}

var mainMenu = Menu{
	Name: "GOTOGEN MENU",
	Items: []Item{
		&Menu{
			Name: "Submenu 1",
			Items: []Item{
				&Menu{
					Name: "Sub-submenu 1",
					Items: []Item{
						ActionItem{
							Name:   "sub-submenu 1 action 1",
							Invoke: func() { println("action pressed") },
						},
						ActionItem{
							Name:   "sub-submenu 1 action 2",
							Invoke: func() { println("action pressed") },
						},
						&SettingItem{
							Name: "sub-submenu 1 setting 1",
						},
					},
				},
				ActionItem{
					Name:   "submenu 1 action 1",
					Invoke: func() { println("action pressed") },
				},
			},
		},
		&Menu{
			Name: "Submenu 2",
			Items: []Item{
				&SettingItem{
					Name: "submenu 2 setting 1",
				},
				&SettingItem{
					Name: "submenu 2 setting 2",
				},
			},
		},
		ActionItem{
			Name:   "Action 1",
			Invoke: func() { println("action pressed") },
		},
		&SettingItem{
			Name:    "Setting 1",
			Active:  1,
			Default: 1,
			Options: []string{"A", "B", "C", "D", "E", "F", "G", "H"},
		},
		&SettingItem{
			Name: "Setting 2",
		},
		&SettingItem{
			Name: "Setting 3",
		},
		&SettingItem{
			Name: "Setting 4",
		},
		&SettingItem{
			Name: "Setting 5",
		},
		&SettingItem{
			Name: "Setting 6",
		},
	},
}
