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
	_ = buf.SetLineInverse(0, m.Name)
	for i := uint8(0); i+m.top < uint8(len(m.Items)) && i < uint8(h-1); i++ {
		item := m.Items[i+m.top]
		var prefix string
		switch item.(type) {
		case *Menu:
			prefix = "+"
		case *ActionItem:
			prefix = "*"
		case *SettingItem:
			prefix = ">"
		}
		if i == m.selected-m.top {
			_ = buf.SetLineInverse(int16(i+1), prefix+item.name())
		} else {
			_ = buf.SetLine(int16(i+1), prefix+item.name())
		}
	}
}

type ActionItem struct {
	Name   string
	Invoke func()
}

func (i *ActionItem) name() string {
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
	Apply    func(selected uint8)
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
	_ = buf.SetLineInverse(0, si.Name)
	for i := uint8(0); i+si.top < uint8(len(si.Options)) && i < uint8(h-1); i++ {
		item := si.Options[i+si.top]
		prefix := " "
		if i == si.Active-si.top {
			prefix = "*"
		}
		if i == si.selected-si.top {
			_ = buf.SetLineInverse(int16(i+1), prefix+item)
		} else {
			_ = buf.SetLine(int16(i+1), prefix+item)
		}
	}
}

type MenuProvider interface {
	GetMenu() Menu
}
