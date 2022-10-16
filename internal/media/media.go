package media

import (
	"embed"
	"errors"
	"image"

	"golang.org/x/image/bmp"
)

//go:embed media/*/*.bmp
var imgs embed.FS

// LoadImage loads the specified image of the specified type.
func LoadImage(typ Type, name string) (image.Image, error) {
	r, err := imgs.Open("media/" + string(typ) + "/" + name + ".bmp")
	if err != nil {
		return nil, err
	}

	fi, err := r.Stat()
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return nil, errors.New("cannot open directory")
	}

	w, h := typ.size()
	if w == 0 || h == 0 {
		return nil, errors.New("invalid media type")
	}

	img, err := bmp.Decode(r)
	if err != nil {
		return nil, err
	}

	b := img.Bounds()
	iw, ih := b.Max.X-b.Min.X, b.Max.Y-b.Min.Y
	if w != iw || h != ih {
		return nil, errors.New("invalid image size for type " + string(typ))
	}

	return img, nil
}
