package mirror

import (
	"image/color"

	"tinygo.org/x/drivers"
)

type Display interface {
	drivers.Displayer

	CanUpdateNow() bool
}

type Mirror struct {
	d     Display
	realW int16
	w, h  int16
}

func New(d Display) *Mirror {
	w, h := d.Size()
	return &Mirror{
		d:     d,
		realW: w,
		w:     w / 2,
		h:     h,
	}
}

func (m *Mirror) Size() (x, y int16) {
	return m.w, m.h
}

func (m *Mirror) SetPixel(x, y int16, c color.RGBA) {
	m.d.SetPixel(x, y, c)
	m.d.SetPixel(m.realW-x-1, y, c)
}

func (m *Mirror) Display() error {
	return m.d.Display()
}

func (m *Mirror) CanUpdateNow() bool {
	return m.d.CanUpdateNow()
}
