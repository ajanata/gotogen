package static

import (
	"image"

	"tinygo.org/x/drivers"

	"github.com/ajanata/gotogen/internal/animation"
	"github.com/ajanata/gotogen/internal/media"
)

type Anim struct {
	img image.Image
}

func New(file string) (*Anim, error) {
	img, err := media.LoadImage(media.TypeFull, file)
	if err != nil {
		return nil, err
	}

	return &Anim{
		img: img,
	}, nil
}

func (a *Anim) Activate(disp drivers.Displayer) {
	animation.DrawImage(disp, 0, 0, a.img, false)
}

func (a *Anim) DrawFrame(_ drivers.Displayer, _ uint32) bool { return true }
