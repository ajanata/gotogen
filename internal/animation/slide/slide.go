package slide

import (
	"image"

	"tinygo.org/x/drivers"

	"github.com/ajanata/gotogen/internal/animation"
	"github.com/ajanata/gotogen/internal/media"
)

type Anim struct {
	img image.Image
	x   int16
}

func New(file string) (animation.Animation, error) {
	img, err := media.LoadImage(media.TypeFull, file)
	if err != nil {
		return nil, err
	}

	return &Anim{
		img: img,
	}, nil
}

func (a *Anim) Activate(_ drivers.Displayer) {
	a.x = 0
}

func (a *Anim) DrawFrame(disp drivers.Displayer, _ uint32) bool {
	w, _ := disp.Size()
	animation.DrawImage(disp, a.x, 0, a.img, true)
	a.x++
	if a.x >= w {
		a.x = 0
	}
	return true
}
