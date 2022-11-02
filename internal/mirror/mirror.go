package mirror

import (
	"image/color"

	"tinygo.org/x/drivers"
)

type Mirror struct {
	d     drivers.Displayer
	realW int16
	w, h  int16
}

var _ drivers.Displayer = (*Mirror)(nil)

func New(d drivers.Displayer) Mirror {
	w, h := d.Size()
	return Mirror{
		d:     d,
		realW: w,
		w:     w / 2,
		h:     h,
	}
}

func (m Mirror) Size() (x, y int16) {
	return m.w, m.h
}

func (m Mirror) SetPixel(x, y int16, c color.RGBA) {
	m.d.SetPixel(x, y, c)
	m.d.SetPixel(m.realW-x-1, y, c)
}

func (m Mirror) Display() error {
	return m.d.Display()
}
