package gotogen

import (
	"errors"
	"image"
	"image/color"
	"runtime"
	"time"

	"github.com/ajanata/textbuf"
	"tinygo.org/x/drivers"

	"github.com/ajanata/gotogen/internal/media"
)

type Gotogen struct {
	framerate   uint
	faceDisplay drivers.Displayer
	boopSensor  BoopSensor
	status      Blinker

	menuDisplay drivers.Displayer
	menuInput   MenuInput
	menuText    *textbuf.Buffer // TODO interface

	initter Initter
	log     Logger

	tick uint32
	init bool
}

// Initter is a function to initialize secondary devices after the primary menu display has been initialized for boot
// messages. Hardware drivers shall configure any buses (SPI, etc.) that are required to communicate with these devices
// at this point, and should only configure the bare minimum to call New.
type Initter func() (faceDisplay drivers.Displayer, menuInput MenuInput, boopSensor BoopSensor, err error)

type Blinker interface {
	Low()
	High()
}

type BoopSensor interface {
	SettingProvider

	BoopDistance() uint8
}

type MenuButton uint8

const (
	MenuButtonNone = iota
	MenuButtonMenu
	MenuButtonBack
	MenuButtonUp
	MenuButtonDown
	// MenuButtonReset is for resetting a specific setting to its default value, and must be held for at least 1 second.
	MenuButtonReset
)

type MenuInput interface {
	// PressedButton returns the currently-pressed menu button. The implementation is responsible for prioritizing
	// multiple buttons being pressed at the same time however it sees fit (or implement some buttons as a chord of
	// multiple physical buttons), as well as handling debouncing (if needed).
	//
	// This function should expect to be called at the main loop framerate.
	PressedButton() MenuButton
}

func New(framerate uint, log Logger, menu drivers.Displayer, status Blinker, initter Initter) (*Gotogen, error) {
	if framerate == 0 {
		return nil, errors.New("must run at least one frame per second")
	}
	if log == nil {
		log = stderrLogger{}
	}
	if menu == nil {
		return nil, errors.New("must provide menu display")
	}
	if initter == nil {
		return nil, errors.New("must provide initter")
	}

	return &Gotogen{
		framerate:   framerate,
		menuDisplay: menu,
		status:      status,
		log:         log,
		initter:     initter,
	}, nil
}

func (g *Gotogen) Init() error {
	if g.init {
		return errors.New("already initialized")
	}
	g.log.Info("starting init")
	g.blink()

	var err error
	// TODO font size configurable
	g.menuText, err = textbuf.New(g.menuDisplay, textbuf.FontSize6x8)
	if err != nil {
		return errors.New("init menu: " + err.Error())
	}

	w, h := g.menuText.Size()
	if w < 15 || h < 4 {
		return errors.New("unusably small menu display")
	}

	err = g.menuText.SetLineInverse(0, "GOTOGEN BOOTING")
	if err != nil {
		return errors.New("boot msg: " + err.Error())
	}
	// we already validated it has at least 4 lines
	_ = g.menuText.SetY(1)
	// we already know it was possible to print text so don't bother checking every time
	_ = g.menuText.Print("Initialize devices")

	faceDisplay, menuInput, boopSensor, err := g.initter()
	if err != nil {
		g.menuText.PrintlnInverse(err.Error())
		return errors.New("initter: " + err.Error())
	}
	// if faceDisplay == nil {
	// 	return errors.New("initter did not provide face")
	// }
	// if menuInput == nil {
	// 	return errors.New("initter did not provide menu input")
	// }

	g.faceDisplay = faceDisplay
	g.menuInput = menuInput
	g.boopSensor = boopSensor
	g.initter = nil
	_ = g.menuText.Println(".")
	_ = g.menuText.Println("The time is now")
	_ = g.menuText.Println(time.Now().Format(time.Stamp))
	_ = g.menuText.Println("Gotogen online.")

	if faceDisplay != nil {
		faceW, faceH := faceDisplay.Size()
		g.log.Debugf("face size = %d x %d", faceW, faceH)

		cant, err := media.LoadImage(media.TypeFull, "Elbrarmemestickerscant")
		if err != nil {
			return err
		}
		b := cant.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, b, a := cant.At(x, y).RGBA()
				faceDisplay.SetPixel(int16(x), int16(y), color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)})
				faceDisplay.SetPixel(int16(x+64), int16(31-y), color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)})
			}
		}

		go g.faceDisplayLoop()
	}

	g.blink()
	g.init = true
	g.log.Info("init complete")
	return nil
}

func (g *Gotogen) faceDisplayLoop() {
	for {
		err := g.faceDisplay.Display()
		if err != nil {
			g.panic(err)
		}
		runtime.Gosched()
	}
}

// Run does not return. It attempts to run the main loop at the framerate specified in New.
func (g *Gotogen) Run() {
	var err error
	cant, err = media.LoadImage(media.TypeFull, "Elbrarmemestickerscant")
	if err != nil {
		g.panic(err)
	}

	// I'd rather range time.Tick but that seems to be less performant
	for {
		err := g.RunTick()
		if err != nil {
			g.panic(err)
		}
		time.Sleep(time.Second / time.Duration(g.framerate))
	}
}

var imgX int16

var cant image.Image

// RunTick runs a single iteration of the main loop.
func (g *Gotogen) RunTick() error {
	if !g.init {
		return errors.New("not initialized")
	}
	println("tick")

	start := time.Now()
	g.statusOn()
	g.tick++

	if g.tick%uint32(g.framerate) == 0 {
		g.menuText.SetLine(7, time.Now().Format(time.Stamp))
	}

	b := cant.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, gr, b, a := cant.At(x, y).RGBA()
			g.faceDisplay.SetPixel((128-int16(x)+imgX)%128, int16(31-y), color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})
		}
	}
	for y := int16(0); y < 31; y++ {
		g.faceDisplay.SetPixel((128-64+imgX)%128, y, color.RGBA{})
	}
	imgX++
	if imgX == 128 {
		imgX = 0
	}

	// if g.tick%2 == 0 {
	// 	cant, err := media.LoadImage(media.TypeFull, "Elbrarmemestickerscant")
	// 	if err != nil {
	// 		return err
	// 	}
	// 	b := cant.Bounds()
	// 	for y := b.Min.Y; y < b.Max.Y; y++ {
	// 		for x := b.Min.X; x < b.Max.X; x++ {
	// 			r, gr, b, a := cant.At(x, y).RGBA()
	// 			g.faceDisplay.SetPixel(int16(x), int16(y), color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})
	// 			g.faceDisplay.SetPixel(int16(x+64), int16(31-y), color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})
	// 		}
	// 	}
	// } else {
	// 	eyes, err := media.LoadImage(media.TypeFull, "Elbrarstickerswatching")
	// 	if err != nil {
	// 		return err
	// 	}
	// 	b := eyes.Bounds()
	// 	for y := b.Min.Y; y < b.Max.Y; y++ {
	// 		for x := b.Min.X; x < b.Max.X; x++ {
	// 			r, gr, b, a := eyes.At(x, y).RGBA()
	// 			g.faceDisplay.SetPixel(int16(x), int16(y), color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})
	// 			g.faceDisplay.SetPixel(int16(x+64), int16(31-y), color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})
	// 		}
	// 	}
	// }

	g.statusOff()
	frameTime := time.Now().Sub(start)
	g.log.Debugf("frame time %s", frameTime.String())
	return nil
}

// unfortunately you can't recover runtime panics in tinygo, so this is just going to be used for things we detect
// that are fatal
func (g *Gotogen) panic(v any) {
	println(v)
	g.log.Infof("%v", v)
	if g.status == nil {
		panic(v)
	}
	for {
		println(v)
		g.blink()
	}
}

func (g *Gotogen) blink() {
	g.statusOn()
	time.Sleep(100 * time.Millisecond)
	g.statusOff()
	time.Sleep(100 * time.Millisecond)
}

func (g *Gotogen) statusOn() {
	if g.status != nil {
		g.status.High()
	}
}

func (g *Gotogen) statusOff() {
	if g.status != nil {
		g.status.Low()
	}
}
