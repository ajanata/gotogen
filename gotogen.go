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

type SensorStatus uint8

const (
	// SensorStatusUnavailable indicates that the sensor is never available (not implemented in hardware).
	SensorStatusUnavailable = iota
	// SensorStatusAvailable indicates that the returned value(s) is/are accurate.
	SensorStatusAvailable
	// SensorStatusBusy indicates that the sensor is temporarily unavailable e.g. due to bus contention.
	SensorStatusBusy
)

type Gotogen struct {
	framerate   uint
	frameTime   time.Duration
	faceDisplay drivers.Displayer
	faceMirror  drivers.Displayer
	status      Blinker
	boopDist    uint8
	aX, aY, aZ  int32 // accelerometer

	menuDisplay     drivers.Displayer
	menuMirror      drivers.Displayer
	menuText        *textbuf.Buffer // TODO interface
	menuState       statusState
	menuStateChange time.Time
	rootMenu        Menu
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
	EarlyInit() (faceDisplay drivers.Displayer, err error)

	// LateInit performs any late initialization (e.g. connecting to wifi to set the clock). The failure of anything in
	// LateInit should not cause the failure of the entire process. Boot messages may be freely logged.
	//
	// TODO interface
	LateInit(buffer *textbuf.Buffer)

	// PressedButton returns the currently-pressed menu button. The implementation is responsible for prioritizing
	// multiple buttons being pressed at the same time however it sees fit (or implement some buttons as a chord of
	// multiple physical buttons), as well as handling debouncing (if needed) and button repeating. Basically, this
	// should only return a value when that value should be acted upon.
	//
	// This function should expect to be called at the main loop framerate.
	PressedButton() MenuButton

	// MenuItems is invoked every time the menu is displayed to retrieve the current menu items for the driver. The
	// driver may return different menu items depending on current state.
	MenuItems() []Item

	// BoopDistance is a normalized value for the closeness of a boop. TODO define the normalization
	// The second return value indicates the status of the boop sensor: does not exist, valid data, or busy.
	BoopDistance() (uint8, SensorStatus)

	// Accelerometer is a normalized value for accelerometer values. When not in motion,
	// all values should be approximately zero. Drivers should provide a calibration option to zero out the sensor.
	// The second return value indicates the status of the accelerometer: does not exist, valid data, or busy.
	Accelerometer() (x, y, z int32, status SensorStatus)
}

type Blinker interface {
	Low()
	High()
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

	faceDisplay, err := g.driver.EarlyInit()
	if err != nil {
		_ = g.menuText.PrintlnInverse(err.Error())
		return errors.New("early init: " + err.Error())
	}
	if faceDisplay == nil {
		return errors.New("init did not provide face")
	}

	g.faceDisplay = faceDisplay
	g.faceMirror = mirror.New(faceDisplay)
	_ = g.menuText.Println(".")

	// now that we have the face panels set up, we can put a loading image on them while LateInit runs
	err = g.busy()
	if err != nil {
		_ = g.menuText.PrintlnInverse("load busy: " + err.Error())
		return errors.New("load busy: " + err.Error())
	}

	_ = g.menuText.Println("CPUs: " + strconv.Itoa(runtime.NumCPU()))
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	g.totalRAM = strconv.Itoa(int(mem.HeapSys / 1024))
	_ = g.menuText.Println(strconv.Itoa(int(mem.HeapSys/1024)) + "k RAM, " + strconv.Itoa(int(mem.HeapIdle/1024)) + "k free")

	g.driver.LateInit(g.menuText)
	g.initMainMenu()

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

	// redraw := false
	if time.Since(g.lastSec) >= time.Second {
		g.lastFPS = g.tick - g.lastTicks
		g.lastSec = time.Now()
		g.lastTicks = g.tick
		// redraw = true
	}

	// read sensors
	d, st := g.driver.BoopDistance()
	if st == SensorStatusAvailable {
		g.boopDist = d
	}

	x, y, z, st := g.driver.Accelerometer()
	if st == SensorStatusAvailable {
		x /= 1000
		y /= 1000
		z /= 1000
		g.aX, g.aY, g.aZ = x, y, z
	}

	g.updateStatus(st == SensorStatusAvailable)

	// TODO begin temporary animation for performance testing

	if moveImg {
		g.SetFullFace(cant, imgX, 0, st == SensorStatusAvailable)
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
	_ = g.menuText.SetLine(0, time.Now().Format("03:04"), " ", strconv.Itoa(int(g.lastFPS)), "Hz ", strconv.Itoa(int(mem.HeapIdle/1024)), "k/", g.totalRAM, "k")

	// TODO temp hack
	_ = g.menuText.SetLine(1, strconv.Itoa(int(g.boopDist)), " ", strconv.Itoa(int(g.aX)), " ", strconv.Itoa(int(g.aY)), " ", strconv.Itoa(int(g.aZ)))
	_ = g.menuText.SetY(2)
	// _ = g.menuText.Println("TODO: more stuff, like at least indicating what states are being displayed outside")
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
				// in case a menu is empty for some reason
				if len(active.Items) == 0 || int(active.selected) > len(active.Items) {
					break
				}
				switch item := active.Items[active.selected].(type) {
				case *Menu:
					item.prev, g.activeMenu = g.activeMenu, item
					item.Render(g.menuText)
				case *ActionItem:
					item.Invoke()
				case *SettingItem:
					item.prev, g.activeMenu = g.activeMenu, item
					item.selected = item.Active
					_, h := g.menuText.Size()
					if item.selected > item.top+uint8(h)-2 {
						// TODO avoid empty lines at the bottom?
						item.top = item.selected
					}
					item.Render(g.menuText)
				}
			case *SettingItem:
				active.Active = active.selected
				active.Apply(active.selected)
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
		if g.driver.PressedButton() != MenuButtonNone {
			g.changeStatusState(statusStateIdle)
		}
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
		m := g.rootMenu.Items[0].(*Menu)
		m.Items = g.driver.MenuItems()
		g.activeMenu = &g.rootMenu
		g.rootMenu.Render(g.menuText)
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

func (g *Gotogen) SetFullFace(img image.Image, offX, offY int16, drawMenu bool) {
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		xx := (64 - int16(x) + offX) % 64
		for y := b.Min.Y; y < b.Max.Y; y++ {
			r, gr, b, a := img.At(x, y).RGBA()
			// FIXME remove hardcoded dimensions and origin flip
			g.faceMirror.SetPixel(xx, (int16(y)+offY)%32, color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)})

			if drawMenu {
				// mirror to menu
				// RGBA returns each channel |= itself << 8 for whatever reason
				r &= 0xFF
				if r < 0xA0 {
					r = 0
				}
				// FIXME remove hardcoded dimensions
				g.menuMirror.SetPixel(xx, 32+(int16(y)+offY)%32, color.RGBA{uint8(r), 0, 0, 1})
			}
		}
	}
}

func (g *Gotogen) initMainMenu() {
	g.rootMenu = Menu{
		Name: "GOTOGEN MENU",
		Items: []Item{
			// This must be first, and will be filled in by the driver's menu items every time the menu is displayed.
			&Menu{
				Name: "Hardware Settings",
			},
			&ActionItem{
				Name:   "Blank status screen",
				Invoke: func() { g.changeStatusState(statusStateBlank) },
			},
			&Menu{
				Name: "Submenu 1",
				Items: []Item{
					&Menu{
						Name: "Sub-submenu 1",
						Items: []Item{
							&ActionItem{
								Name:   "sub-submenu 1 action 1",
								Invoke: func() { println("action pressed") },
							},
							&ActionItem{
								Name:   "sub-submenu 1 action 2",
								Invoke: func() { println("action pressed") },
							},
							&SettingItem{
								Name:  "sub-submenu 1 setting 1",
								Apply: func(uint8) { println("setting saved") },
							},
						},
					},
					&ActionItem{
						Name:   "submenu 1 action 1",
						Invoke: func() { println("action pressed") },
					},
				},
			},
			&Menu{
				Name: "Submenu 2",
				Items: []Item{
					&SettingItem{
						Name: "submenu 2 setting 1",
					},
					&SettingItem{
						Name: "submenu 2 setting 2",
					},
				},
			},
			&ActionItem{
				Name:   "Action 1",
				Invoke: func() { println("action pressed") },
			},
			&SettingItem{
				Name:    "Setting 1",
				Active:  1,
				Default: 1,
				Options: []string{"A", "B", "C", "D", "E", "F", "G", "H"},
				Apply:   func(s uint8) { println("setting saved " + strconv.Itoa(int(s))) },
			},
			&SettingItem{
				Name:  "Setting 2",
				Apply: func(uint8) { println("setting saved") },
			},
			&SettingItem{
				Name:  "Setting 3",
				Apply: func(uint8) { println("setting saved") },
			},
			&SettingItem{
				Name:  "Setting 4",
				Apply: func(uint8) { println("setting saved") },
			},
			&SettingItem{
				Name:  "Setting 5",
				Apply: func(uint8) { println("setting saved") },
			},
			&SettingItem{
				Name:  "Setting 6",
				Apply: func(uint8) { println("setting saved") },
			},
		},
	}
}

func (g *Gotogen) Busy(f func(buffer *textbuf.Buffer)) {
	g.menuText.AutoFlush = true
	g.menuText.Clear()

	err := g.busy()
	if err != nil {
		print("unable to load busy", err)
		_ = g.menuText.PrintlnInverse("loading busy: " + err.Error())
	}
	f(g.menuText)

	s := time.Now()
	for time.Now().Before(s.Add(5 * time.Second)) {
		if g.driver.PressedButton() != MenuButtonNone {
			break
		}
	}

	g.menuText.AutoFlush = false
	g.changeStatusState(statusStateIdle)
}

func (g *Gotogen) busy() error {
	busy, err := media.LoadImage(media.TypeFull, "wait")
	if err != nil {
		return errors.New("load busy: " + err.Error())
	}
	g.SetFullFace(busy, 0, 0, false)
	_ = g.faceDisplay.Display()
	return nil
}
