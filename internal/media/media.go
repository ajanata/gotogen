package media

import (
	"embed"
	"errors"
	"tinygo.org/x/drivers/image/png"
)

//go:embed media/*/*.png
var imgs embed.FS

// LoadImage loads the specified image of the specified type.
//
// NOTE: img is returned indexed by rows then columns (y, then x).
func LoadImage(typ Type, name string) (w, h int16, img [][]RGB565, err error) {
	r, err := imgs.Open("media/" + string(typ) + "/" + name + ".png")
	if err != nil {
		return 0, 0, nil, err
	}
	fi, err := r.Stat()
	if err != nil {
		return 0, 0, nil, err
	}
	if fi.IsDir() {
		return 0, 0, nil, errors.New("cannot open directory")
	}
	w, h = typ.Size()
	if w == 0 || h == 0 {
		return 0, 0, nil, errors.New("invalid media type")
	}

	img = make([][]RGB565, h)
	buf := make([]uint16, w)
	png.SetCallback(buf, func(data []uint16, x, y, w, h, width, height int16) {
		// TODO data validation
		img[y] = make([]RGB565, w)
		for i, d := range data {
			img[y][i] = RGB565(d)
		}
	})
	_, err = png.Decode(r)
	return w, h, img, err
}
