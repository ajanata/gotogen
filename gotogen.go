package gotogen

import (
	"errors"
	"image"
	"image/color"
	"runtime"
	"strconv"
	"time"

	"github.com/ajanata/textbuf"
	"tinygo.org/x/drivers"

	"github.com/ajanata/gotogen/internal/media"
	"github.com/ajanata/gotogen/internal/mirror"
)

const menuTimeout = 10 * time.Second

type Gotogen struct {
	framerate   uint
	frameTime   time.Duration
	faceDisplay drivers.Displayer
	faceMirror  drivers.Displayer
	boopSensor  BoopSensor
	status      Blinker

	menuDisplay     drivers.Displayer
	menuMirror      drivers.Displayer
	menuText        *textbuf.Buffer // TODO interface
	menuState       statusState
	menuStateChange time.Time
	activeMenu      Menuable

	driver Driver

	init  bool
	start time.Time

	tick      uint32
	lastSec   time.Time
	lastTicks uint32
	lastFPS   uint32

	// storing this once could be inaccurate on OS-based implementations, but you also don't really care in that case
	totalRAM string
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
	MenuProvider

	BoopDistance() uint8
}

func New(framerate uint, menu drivers.Displayer, status Blinker, driver Driver) (*Gotogen, error) {
	if framerate == 0 {
		return nil, errors.New("must run at least one frame per second")
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
		driver:      driver,
		start:       time.Now(),
	}, nil
}

func (g *Gotogen) Init() error {
	if g.init {
		return errors.New("already initialized")
	}
	println("starting init")
	g.blink()

	var err error
	// TODO font size configurable
	g.menuText, err = textbuf.New(g.menuDisplay, textbuf.FontSize6x8)
	if err != nil {
		return errors.New("init menu: " + err.Error())
	}
	g.menuText.AutoFlush = true

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

	_ = g.menuText.Println("CPUs: " + strconv.Itoa(runtime.NumCPU()))
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	g.totalRAM = strconv.Itoa(int(mem.HeapSys / 1024))
	_ = g.menuText.Println(strconv.Itoa(int(mem.HeapSys/1024)) + "k RAM, " + strconv.Itoa(int(mem.HeapIdle/1024)) + "k free")

	err = g.driver.LateInit(g.menuText)
	if err != nil {
		_ = g.menuText.PrintlnInverse("late init: " + err.Error())
		// LateInit is not allowed to cause the boot to fail
	}

	_ = g.menuText.Println("The time is now")
	_ = g.menuText.Println(time.Now().Format(time.Stamp))
	_ = g.menuText.Println("Booted in " + time.Now().Sub(g.start).Round(100*time.Millisecond).String())
	_ = g.menuText.Println("Gotogen online.")

	g.menuText.AutoFlush = false
	g.menuStateChange = time.Now()

	g.blink()
	g.init = true
	println("init complete in", time.Now().Sub(g.start).Round(100*time.Millisecond).String())
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

	g.statusOff()
	g.tick++

	redraw := false
	if time.Since(g.lastSec) >= time.Second {
		g.lastFPS = g.tick - g.lastTicks
		g.lastSec = time.Now()
		g.lastTicks = g.tick
		redraw = true
	}

	g.updateStatus(redraw)

	// TODO begin temporary animation for performance testing

	if moveImg {
		g.SetFullFace(cant, imgX, 0)
		imgX++
		if imgX == 64 {
			imgX = 0
		}
	}

	// TODO end temporary animation for performance testing

	err := g.faceDisplay.Display()
	if err != nil {
		g.panic(err)
	}

	if g.menuState != statusStateBlank {
		err = g.menuText.Display()
		if err != nil {
			g.panic(err)
		}
	}

	g.statusOn()
	return nil
}

func (g *Gotogen) drawIdleStatus() {
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	// TODO switch which line this is on every minute or so for burn-in protection
	_ = g.menuText.SetLine(0, time.Now().Format("03:04")+" "+strconv.Itoa(int(g.lastFPS))+"Hz "+strconv.Itoa(int(mem.HeapIdle/1024))+"k/"+g.totalRAM+"k")
	_ = g.menuText.SetY(1)
	_ = g.menuText.Println("TODO: more stuff, like at least indicating what states are being displayed outside")
}

func (g *Gotogen) updateStatus(redraw bool) {
	switch g.menuState {
	case statusStateBoot:
		if time.Now().After(g.menuStateChange.Add(menuTimeout)) {
			g.changeStatusState(statusStateIdle)
			break
		}
		// any button press clears the boot log
		if g.driver.PressedButton() != MenuButtonNone {
			g.changeStatusState(statusStateIdle)
		}
	case statusStateIdle:
		if g.driver.PressedButton() == MenuButtonMenu {
			g.changeStatusState(statusStateMenu)
			break
		}

		if redraw {
			g.drawIdleStatus()
		}
	case statusStateMenu:
		if time.Now().After(g.menuStateChange.Add(menuTimeout)) {
			g.changeStatusState(statusStateIdle)
			break
		}

		switch g.driver.PressedButton() {
		case MenuButtonBack:
			g.menuStateChange = time.Now()
			if g.activeMenu.Prev() == nil {
				// at top level menu
				g.changeStatusState(statusStateIdle)
			} else {
				m := g.activeMenu
				g.activeMenu = g.activeMenu.Prev()
				m.SetPrev(nil)
				g.activeMenu.Render(g.menuText)
			}
		case MenuButtonMenu:
			g.menuStateChange = time.Now()
			switch active := g.activeMenu.(type) {
			case *Menu:
				switch item := active.Items[active.selected].(type) {
				case *Menu:
					item.prev, g.activeMenu = g.activeMenu, item
					item.Render(g.menuText)
				case ActionItem:
					item.Invoke()
				case *SettingItem:
					item.prev, g.activeMenu = g.activeMenu, item
					item.selected = item.Active
					_, h := g.menuText.Size()
					if item.selected > item.top+uint8(h)-2 {
						// TODO avoid empty lines at the bottom
						item.top = item.selected
					}
					item.Render(g.menuText)
				}
			case *SettingItem:
				active.Active = active.selected
				g.activeMenu, active.prev = active.prev, nil
				g.activeMenu.Render(g.menuText)
			}
		case MenuButtonUp:
			g.menuStateChange = time.Now()
			if g.activeMenu.Selected() > 0 {
				g.activeMenu.SetSelected(g.activeMenu.Selected() - 1)
			}
			if g.activeMenu.Selected() < g.activeMenu.Top() {
				g.activeMenu.SetTop(g.activeMenu.Selected())
			}
			g.activeMenu.Render(g.menuText)
		case MenuButtonDown:
			g.menuStateChange = time.Now()
			g.activeMenu.SetSelected(g.activeMenu.Selected() + 1)
			if g.activeMenu.Selected() > g.activeMenu.Len()-1 {
				g.activeMenu.SetSelected(g.activeMenu.Len() - 1)
			}
			_, h := g.menuText.Size()
			if g.activeMenu.Selected() > g.activeMenu.Top()+uint8(h)-2 {
				g.activeMenu.SetTop(g.activeMenu.Top() + 1)
			}
			g.activeMenu.Render(g.menuText)
		}
	case statusStateBlank:
		// nothing to do
	}
}

func (g *Gotogen) changeStatusState(state statusState) {
	switch state {
	case statusStateIdle:
		g.activeMenu = nil
		g.menuStateChange = time.Now()
		g.menuText.Clear()
		g.drawIdleStatus()
	case statusStateBlank:
		g.activeMenu = nil
		// clear text buffer
		g.menuText.Clear()
		// but make sure we clear the *entire* screen, including pixels outside the coverage of the text buffer
		w, h := g.menuDisplay.Size()
		for x := int16(0); x < w; x++ {
			for y := int16(0); y < h; y++ {
				g.menuDisplay.SetPixel(x, y, color.RGBA{})
			}
		}
		// since it won't be drawn in the main loop
		_ = g.menuDisplay.Display()
	case statusStateMenu:
		g.activeMenu = &mainMenu
		mainMenu.Render(g.menuText)
	}
	g.menuState = state
	g.menuStateChange = time.Now()
}

// unfortunately you can't recover runtime panics in tinygo, so this is just going to be used for things we detect
// that are fatal
func (g *Gotogen) panic(v any) {
	println(v)
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
