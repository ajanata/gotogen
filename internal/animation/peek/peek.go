package peek

import (
	"image"
	"image/color"
	"time"

	"tinygo.org/x/drivers"

	"github.com/ajanata/gotogen/internal/animation"
	"github.com/ajanata/gotogen/internal/media"
)

type Anim struct {
	img    image.Image
	y      int16
	next   time.Time
	moveUp bool
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

func (a *Anim) Activate(disp drivers.Displayer) {
	a.y = int16(-a.img.Bounds().Max.Y)
	a.next = time.Now()
	a.moveUp = false

	// blank the screen
	w, h := disp.Size()
	for x := int16(0); x < w; x++ {
		for y := int16(0); y < h; y++ {
			disp.SetPixel(x, y, color.RGBA{})
		}
	}
}

func (a *Anim) DrawFrame(disp drivers.Displayer, _ uint32) bool {
	if time.Now().Before(a.next) {
		return true
	}
	if a.moveUp {
		// blank the row we're moving up from
		y := a.y + int16(a.img.Bounds().Max.Y) - 1
		if y >= 0 {
			for x := int16(0); x < int16(a.img.Bounds().Max.X); x++ {
				disp.SetPixel(x, y, color.RGBA{})
			}
		}
		a.y--
	} else {
		a.y++
	}
	animation.DrawImage(disp, 0, a.y, a.img, false)
	if a.y >= 0 {
		a.next = time.Now().Add(3 * time.Second)
		a.moveUp = true
	} else if a.y < int16(-a.img.Bounds().Max.Y) {
		return false
	} else {
		a.next = time.Now().Add(100 * time.Millisecond)
	}
	return true
}
