package gotogen

import (
	"errors"
	"image"
	"image/color"
	"runtime"
	"strconv"
	"time"

	"github.com/ajanata/textbuf"

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
	framerate  uint
	frameTime  time.Duration
	status     Blinker
	boopDist   uint8
	aX, aY, aZ int32 // accelerometer

	faceDisplay Display
	faceMirror  Display
	faceState   faceState

	menuDisplay              Display
	menuMirror               Display
	menuText                 *textbuf.Buffer // TODO interface
	menuState                statusState
	menuStateChange          time.Time
	menuMirrorSkip           uint8
	menuMirrorDownmixChannel colorChannel
	menuMirrorDownmixCutoff  uint8
	rootMenu                 Menu
	activeMenu               Menuable

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
	EarlyInit() (faceDisplay Display, err error)

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

	// MenuItems is invoked every time the menu is displayed to retrieve the current menu items for the driver.
	// The driver may return different menu items depending on current state.
	MenuItems() []Item

	// BoopDistance is a normalized value for the closeness of a boop. TODO define the normalization
	// The second return value indicates the status of the boop sensor: does not exist, valid data, or busy.
	BoopDistance() (uint8, SensorStatus)

	// Accelerometer is a normalized value for accelerometer values. TODO define the scale of the normalized values
	// When not in motion, all values should be approximately zero.
	// Drivers should provide a calibration option to zero out the sensor.
	// The second return value indicates the status of the accelerometer: does not exist, valid data, or busy.
	Accelerometer() (x, y, z int32, status SensorStatus)
}

type Blinker interface {
	Low()
	High()
}

func New(framerate uint, menu Display, status Blinker, driver Driver) (*Gotogen, error) {
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

	// TODO load from settings storage; this is also defined in initMainMenu
	g.menuMirrorSkip = 4
	g.menuMirrorDownmixCutoff = 0xA0

	g.blink()
	g.init = true
	println("init complete in", time.Now().Sub(g.start).Round(100*time.Millisecond).String())
	return nil
}

// Run does not return. It attempts to run the main loop at the framerate specified in New.
func (g *Gotogen) Run() {
	var err error
	cant, err = media.LoadImage(media.TypeFull, "cant")
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

	// busy states clear when we get back to the run loop
	if g.faceState == faceStateBusy {
		g.faceState = faceStateDefault
	}

	if time.Since(g.lastSec) >= time.Second {
		g.lastFPS = g.tick - g.lastTicks
		g.lastSec = time.Now()
		g.lastTicks = g.tick
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

	// TODO better way to framerate limit the status screen
	canRedrawStatus := g.menuDisplay.CanUpdateNow()
	if g.menuMirrorSkip > 0 {
		canRedrawStatus = canRedrawStatus && uint8(g.tick)%g.menuMirrorSkip == 0
	}
	// we always need to call this tho since the menu handling code is in here
	g.updateStatus(canRedrawStatus)

	// TODO begin temporary animation for performance testing

	if moveImg {
		g.SetFullFace(cant, imgX, 0, canRedrawStatus)
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

	if g.menuState != statusStateBlank && canRedrawStatus {
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

	// println(time.Now().Format("03:04"), g.lastFPS, "Hz", mem.HeapIdle/1024, "k/", g.totalRAM)
}

func (g *Gotogen) updateStatus(updateIdleStatus bool) {
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

		if updateIdleStatus {
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

func (g *Gotogen) clearStatusScreen() {
	// clear text buffer
	g.menuText.Clear()
	// but make sure we clear the *entire* screen, including pixels outside the coverage of the text buffer
	w, h := g.menuDisplay.Size()
	for x := int16(0); x < w; x++ {
		for y := int16(0); y < h; y++ {
			g.menuDisplay.SetPixel(x, y, color.RGBA{})
		}
	}
	_ = g.menuDisplay.Display()
}

func (g *Gotogen) changeStatusState(state statusState) {
	println("changing to status state", state.String())
	g.activeMenu = nil
	g.menuState = state
	g.menuStateChange = time.Now()
	g.clearStatusScreen()

	switch state {
	case statusStateIdle:
		g.drawIdleStatus()
	case statusStateBlank:
		// nothing special to do
	case statusStateMenu:
		// hardware submenu is required to be the first item in the menu
		m := g.rootMenu.Items[0].(*Menu)
		m.Items = g.driver.MenuItems()
		g.activeMenu = &g.rootMenu
		g.rootMenu.Render(g.menuText)
	}
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

func (g *Gotogen) SetFullFace(img image.Image, offX, offY int16, drawStatusMirror bool) {
	drawStatusMirror = drawStatusMirror && g.menuDisplay.CanUpdateNow() && g.menuState == statusStateIdle
	// drawStatusMirror = drawStatusMirror && g.menuState == statusStateIdle

	w, h := g.faceMirror.Size()
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		xx := (w - int16(x) + offX) % w
		for y := b.Min.Y; y < b.Max.Y; y++ {
			r, gr, b, a := img.At(x, y).RGBA()
			g.faceMirror.SetPixel(xx, (int16(y)+offY)%h, color.RGBA{R: uint8(r), G: uint8(gr), B: uint8(b), A: uint8(a)})

			if drawStatusMirror {
				g.downmixForStatus(xx, int16(y)+offY, h, uint8(r), uint8(gr), uint8(b))
			}
		}
	}
}

func (g *Gotogen) downmixForStatus(x, y int16, h int16, r, gr, b uint8) {
	// RGBA returns each channel |= itself << 8 for whatever reason
	switch g.menuMirrorDownmixChannel {
	case red:
		r &= 0xFF
		if r < g.menuMirrorDownmixCutoff {
			r = 0
		}
		gr = 0
		b = 0
	case green:
		gr &= 0xFF
		if gr < g.menuMirrorDownmixCutoff {
			gr = 0
		}
		r = 0
		b = 0
	case blue:
		b &= 0xFF
		if b < g.menuMirrorDownmixCutoff {
			b = 0
		}
		r = 0
		gr = 0
	}
	// FIXME remove hardcoded offset
	g.menuMirror.SetPixel(x, 32+y%h, color.RGBA{R: r, G: gr, B: b, A: 1})
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
			&SettingItem{
				Name:    "Status mirror skip",
				Options: []string{"0", "1", "2", "4", "8", "16"},
				Active:  3, // TODO load from setting storage
				Apply:   g.setMenuMirrorSkip,
			},
			&SettingItem{
				Name:    "Status mirror color",
				Options: []string{"red", "green", "blue"},
				Active:  0,
				Apply:   g.setMenuMirrorColor,
			},
			&SettingItem{
				Name:    "Status mirror cutoff",
				Options: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "A", "B", "D", "E", "F"},
				Active:  9,
				Apply:   g.setMenuMirrorCutoff,
			},
		},
	}
}

func (g *Gotogen) setMenuMirrorCutoff(selected uint8) {
	g.menuMirrorDownmixCutoff = (selected + 1) << 4
}

func (g *Gotogen) setMenuMirrorColor(selected uint8) {
	g.menuMirrorDownmixChannel = colorChannel(selected)
}

func (g *Gotogen) setMenuMirrorSkip(selected uint8) {
	if selected == 0 {
		g.menuMirrorSkip = 0
	} else {
		g.menuMirrorSkip = 1 << (selected - 1)
	}
	println("setting menu mirror skip to", g.menuMirrorSkip)
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
	g.faceState = faceStateBusy

	busy, err := media.LoadImage(media.TypeFull, "wait")
	if err != nil {
		return errors.New("load busy: " + err.Error())
	}
	g.SetFullFace(busy, 0, 0, false)
	_ = g.faceDisplay.Display()
	return nil
}
