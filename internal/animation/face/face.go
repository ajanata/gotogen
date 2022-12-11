package face

import (
	"image"
	"image/color"

	"tinygo.org/x/drivers"

	"github.com/ajanata/gotogen/internal/animation"
	"github.com/ajanata/gotogen/internal/media"
)

type Anim struct {
	eye   image.Image
	nose  image.Image
	mouth image.Image
}

func New() (*Anim, error) {
	eye, err := media.LoadImage(media.TypeEye, "default")
	if err != nil {
		return nil, err
	}
	nose, err := media.LoadImage(media.TypeNose, "default")
	if err != nil {
		return nil, err
	}
	mouth, err := media.LoadImage(media.TypeMouth, "default")
	if err != nil {
		return nil, err
	}

	return &Anim{
		eye:   eye,
		nose:  nose,
		mouth: mouth,
	}, nil
}

func (a *Anim) Activate(disp drivers.Displayer) {
	w, h := disp.Size()
	for x := int16(0); x < w; x++ {
		for y := int16(0); y < h; y++ {
			disp.SetPixel(x, y, color.RGBA{})
		}
	}
}

func (a *Anim) DrawFrame(disp drivers.Displayer, _ uint32) bool {
	w, h := disp.Size()
	// TODO jitter or something, will need other sensors. the face is allowed to be special-cased for those
	animation.DrawImage(disp, 0, 0, a.eye, false)
	nw, _ := media.TypeNose.Size()
	animation.DrawImage(disp, w-nw, 8, a.nose, false)
	_, mh := media.TypeMouth.Size()
	animation.DrawImage(disp, 3, h-mh-1, a.mouth, false)
	return true
}
