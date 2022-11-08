package gotogen

import (
	"errors"
	"image"
	"image/color"
	"strconv"
	"time"

	"github.com/ajanata/textbuf"
	"tinygo.org/x/drivers"

	"github.com/ajanata/gotogen/internal/media"
	"github.com/ajanata/gotogen/internal/mirror"
)

type Gotogen struct {
	framerate   uint
	frameTime   time.Duration
	faceDisplay drivers.Displayer
	faceMirror  drivers.Displayer
	boopSensor  BoopSensor
	status      Blinker

	menuDisplay drivers.Displayer
	menuMirror  drivers.Displayer
	menuText    *textbuf.Buffer // TODO interface
	driver      Driver
	log         Logger

	init  bool
	start time.Time

	tick      uint32
	lastSec   time.Time
	lastTicks uint32
}

type Driver interface {
	// EarlyInit initializes secondary devices after the primary menu display has been initialized for boot
	// messages. Hardware drivers shall configure any buses (SPI, etc.) that are required to communicate with these
	// devices at this point, and should only configure the bare minimum to call New.
	EarlyInit() (faceDisplay drivers.Displayer, boopSensor BoopSensor, err error)

	// LateInit performs any late initialization (e.g. connecting to wifi to set the clock). The failure of anything in
	// LateInit should not cause the failure of the entire process; returning an error is to simplify logging. Boot
	// messages may be freely logged.
	//
	// TODO interface
	LateInit(buffer *textbuf.Buffer) error

	// PressedButton returns the currently-pressed menu button. The implementation is responsible for prioritizing
	// multiple buttons being pressed at the same time however it sees fit (or implement some buttons as a chord of
	// multiple physical buttons), as well as handling debouncing (if needed) and button repeating. Basically, this
	// should only return a value when that value should be acted upon.
	//
	// This function should expect to be called at the main loop framerate.
	PressedButton() MenuButton
}

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
	// MenuButtonDefault is for resetting a specific setting to its default value. Drivers may wish to require this
	// button to be held down for a second before triggering it, or perhaps make it be a chord of up and down.
	MenuButtonDefault
)

func (b MenuButton) String() string {
	switch b {
	case MenuButtonNone:
		return "none"
	case MenuButtonMenu:
		return "menu"
	case MenuButtonBack:
		return "back"
	case MenuButtonUp:
		return "up"
	case MenuButtonDown:
		return "down"
	case MenuButtonDefault:
		return "default"
	default:
		return "INVALID"
	}
}

func New(framerate uint, log Logger, menu drivers.Displayer, status Blinker, driver Driver) (*Gotogen, error) {
	if framerate == 0 {
		return nil, errors.New("must run at least one frame per second")
	}
	if log == nil {
		log = stderrLogger{}
	}
	if menu == nil {
		return nil, errors.New("must provide menu display")
	}
	if driver == nil {
		return nil, errors.New("must provide driver")
	}

	return &Gotogen{
		framerate:   framerate,
		frameTime:   time.Second / time.Duration(framerate),
		menuDisplay: menu,
		menuMirror:  mirror.New(menu),
		status:      status,
		log:         log,
		driver:      driver,
		start:       time.Now(),
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

	faceDisplay, boopSensor, err := g.driver.EarlyInit()
	if err != nil {
		_ = g.menuText.PrintlnInverse(err.Error())
		return errors.New("early init: " + err.Error())
	}
	if faceDisplay == nil {
		return errors.New("init did not provide face")
	}
	// if menuInput == nil {
	// 	return errors.New("init did not provide menu input")
	// }

	g.faceDisplay = faceDisplay
	g.faceMirror = mirror.New(faceDisplay)
	g.boopSensor = boopSensor
	_ = g.menuText.Println(".")

	// now that we have the face panels set up, we can boot a loading image on them while LateInit runs
	busy, err := media.LoadImage(media.TypeFull, "wait")
	if err != nil {
		_ = g.menuText.PrintlnInverse("load busy: " + err.Error())
		return errors.New("load busy: " + err.Error())
	}
	g.SetFullFace(busy, 0, 0)
	_ = g.faceDisplay.Display()

	err = g.driver.LateInit(g.menuText)
	if err != nil {
		_ = g.menuText.PrintlnInverse("late init: " + err.Error())
		// LateInit is not allowed to cause the boot to fail
	}

	_ = g.menuText.Println("The time is now")
	_ = g.menuText.Println(time.Now().Format(time.Stamp))
	_ = g.menuText.Println("Booted in " + time.Now().Sub(g.start).Round(100*time.Millisecond).String())
	_ = g.menuText.Println("Gotogen online.")

	g.blink()
	g.init = true
	g.log.Info("init complete")
	return nil
}

// Run does not return. It attempts to run the main loop at the framerate specified in New.
func (g *Gotogen) Run() {
	var err error
	cant, err = media.LoadImage(media.TypeFull, "Elbrarmemestickerscant")
	if err != nil {
		g.panic(err)
	}
	moveImg = true

	for range time.Tick(g.frameTime) {
		err := g.RunTick()
		if err != nil {
			g.panic(err)
		}
	}
}

var imgX int16

var cant image.Image
var moveImg bool

// RunTick runs a single iteration of the main loop.
func (g *Gotogen) RunTick() error {
	if !g.init {
		return errors.New("not initialized")
	}

	start := time.Now()
	g.statusOff()
	g.tick++

	if time.Since(g.lastSec) >= time.Second {
		// TODO something better
		_ = g.menuText.SetLine(7, time.Now().Format("03:04:05PM")+" "+strconv.Itoa(int(g.tick-g.lastTicks))+"fps")
		g.lastSec = time.Now()
		g.lastTicks = g.tick
	}

	but := g.driver.PressedButton()
	if but != MenuButtonNone {
		_ = g.menuText.SetLine(6, "button: "+but.String())
	}

	if moveImg {
		g.SetFullFace(cant, imgX, 0)
		imgX++
		if imgX == 64 {
			imgX = 0
		}
		// temp hack if mirroring
		g.menuDisplay.Display()
	}

	err := g.faceDisplay.Display()
	if err != nil {
		g.panic(err)
	}
	g.statusOn()
	frameTime := time.Now().Sub(start)
	g.log.Debugf("frame time %s", frameTime.String())
	return nil
}

// unfortunately you can't recover runtime panics in tinygo, so this is just going to be used for things we detect
// that are fatal
func (g *Gotogen) panic(v any) {
	println(v)
	g.log.Infof("%v", v)
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

func (g *Gotogen) SetFullFace(img image.Image, offX, offY int16) {
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		xx := (64 - int16(x) + offX) % 64
		for y := b.Min.Y; y < b.Max.Y; y++ {
			r, gr, b, a := img.At(x, y).RGBA()
			// FIXME remove hardcoded dimensions and origin flip
			g.faceMirror.SetPixel(xx, (int16(y)+offY)%32, color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})
			// g.faceMirror.SetPixel(xx, (int16(31-y)+offY)%32, color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})

			// mirror to menu
			// // RGBA returns each channel |= itself << 8 for whatever reason
			// r &= 0xFF
			// if r < 0xA0 {
			// 	r = 0
			// }
			// // FIXME remove hardcoded dimensions
			// g.menuMirror.SetPixel(xx, (int16(y)+offY)%32, color.RGBA{uint8(r), 0, 0, 1})
		}
	}
}
