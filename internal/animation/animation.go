package animation

import (
	"image"
	"image/color"

	"tinygo.org/x/drivers"
)

type Animation interface {
	// Activate is called when the animation is being started on the display.
	// An animation may be re-used so this should be able to be called more than once.
	// Typically, this would be used to clear the display for an animation that does not cover the entire display,
	// so each frame only has to draw the changed areas.
	Activate(drivers.Displayer)
	// DrawFrame draws the next frame of the animation.
	// The current frame number is provided to allow animations to be keyed off every-x-frames without having to keep
	// track of that themselves.
	// Returns whether the animation should continue.
	DrawFrame(disp drivers.Displayer, tick uint32) bool
}

// TODO register all of them for menu purposes

// DrawImage draws the image on the display at the given coordinates.
// If wrap is true, off-screen coordinates will wrap around to the other side of the display.
// Otherwise, off-screen coordinates will be clipped.
//
// Wrapping negative offsets may not work correctly.
func DrawImage(disp drivers.Displayer, offX, offY int16, img image.Image, wrap bool) {
	w, h := disp.Size()
	b := img.Bounds()
	// Assumes the minimum X and Y will always be 0, 0. This should be the case for 24-bit bitmaps.
	for x := 0; x < b.Max.X; x++ {
		xx := int16(x) + offX
		if xx < 0 || xx >= w {
			if wrap {
				xx = xx % w
			} else {
				continue
			}
		}
		for y := 0; y < b.Max.Y; y++ {
			yy := int16(y) + offY
			if yy < 0 || yy >= h {
				if wrap {
					yy = yy % h
				} else {
					continue
				}
			}
			r, g, b, a := img.At(x, y).RGBA()
			disp.SetPixel(xx, yy, color.RGBA{
				R: uint8(r),
				G: uint8(g),
				B: uint8(b),
				A: uint8(a),
			})
		}
	}
}
