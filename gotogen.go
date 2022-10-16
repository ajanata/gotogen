package gotogen

import (
	"errors"
	"time"
	"tinygo.org/x/drivers"

	"github.com/ajanata/textbuf"
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

	g.blink()
	g.init = true
	return nil
}

// Run does not return. It attempts to run the main loop at the framerate specified in New.
func (g *Gotogen) Run() {
	for range time.Tick(time.Second / time.Duration(g.framerate)) {
		err := g.RunTick()
		if err != nil {
			panic(err)
		}
	}
}

// RunTick runs a single iteration of the main loop.
func (g *Gotogen) RunTick() error {
	if !g.init {
		return errors.New("not initialized")
	}

	start := time.Now()
	g.statusOn()

	g.statusOff()
	frameTime := time.Now().Sub(start)
	g.log.Debugf("frame time %d", frameTime)
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
