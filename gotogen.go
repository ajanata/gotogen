package gotogen

import (
	"errors"
	"image/color"
	"runtime"
	"strconv"
	"time"

	"github.com/ajanata/textbuf"

	"github.com/ajanata/gotogen/internal/animation"
	"github.com/ajanata/gotogen/internal/animation/face"
	"github.com/ajanata/gotogen/internal/animation/peek"
	"github.com/ajanata/gotogen/internal/animation/slide"
	"github.com/ajanata/gotogen/internal/animation/static"
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
	blinker    Blinker
	boopDist   uint8
	aX, aY, aZ int32 // accelerometer

	faceDisplay Display
	faceMirror  Display
	faceState   faceState
	activeAnim  animation.Animation

	statusDisplay        Display
	statusMirror         Display
	statusFrameSkip      uint8
	statusDownmixChannel colorChannel
	statusDownmixCutoff  uint8
	statusText           *textbuf.Buffer // TODO interface
	statusState          statusState
	statusStateChange    time.Time
	rootMenu             Menu
	activeMenu           Menuable
	statusForceUpdate    bool

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
	// When not in motion, all values should be approximately zero. TODO actually implement that
	// Drivers should provide a calibration option to zero out the sensor.
	// The second return value indicates the status of the accelerometer: does not exist, valid data, or busy.
	Accelerometer() (x, y, z int32, status SensorStatus)

	// Talking indicates if the driver has detected speech and the face should animate talking.
	Talking() bool

	// StatusLine returns a textual status indicator that the driver may use for whatever it wishes.
	//
	// For the current hardware implementation of a 128x64 OLED display with the 6x8 font, this cannot be more than 21
	// characters. Other hardware implementations may have different limits, but since the hardware implementation is
	// what is returning this line, it should know better.
	StatusLine() string
}

type Blinker interface {
	Low()
	High()
}

func New(framerate uint, status Display, blinker Blinker, driver Driver) (*Gotogen, error) {
	if framerate == 0 {
		return nil, errors.New("must run at least one frame per second")
	}
	if status == nil {
		return nil, errors.New("must provide status display")
	}
	if driver == nil {
		return nil, errors.New("must provide driver")
	}

	return &Gotogen{
		framerate:     framerate,
		frameTime:     time.Second / time.Duration(framerate),
		statusDisplay: status,
		statusMirror:  mirror.New(status),
		blinker:       blinker,
		driver:        driver,
		start:         time.Now(),
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
	g.statusText, err = textbuf.New(g.statusDisplay, textbuf.FontSize6x8)
	if err != nil {
		return errors.New("init status: " + err.Error())
	}
	g.statusText.AutoFlush = true

	w, h := g.statusDisplay.Size()
	tw, th := g.statusText.Size()
	// TODO make this more graceful
	if tw < 20 || th < 8 || w < 128 || h < 64 {
		return errors.New("unusably small status display")
	}

	err = g.statusText.SetLineInverse(0, "GOTOGEN BOOTING")
	if err != nil {
		return errors.New("boot msg: " + err.Error())
	}
	// we already validated it has at least 4 lines
	_ = g.statusText.SetY(1)
	// we already know it was possible to print text so don't bother checking every time
	_ = g.statusText.Print("Initialize devices")

	faceDisplay, err := g.driver.EarlyInit()
	if err != nil {
		_ = g.statusText.PrintlnInverse(err.Error())
		return errors.New("early init: " + err.Error())
	}
	if faceDisplay == nil {
		return errors.New("init did not provide face")
	}

	g.faceDisplay = faceDisplay
	g.faceMirror = mirror.New(faceDisplay)
	_ = g.statusText.Println(".")

	// now that we have the face panels set up, we can put a loading image on them while LateInit runs
	err = g.busy()
	if err != nil {
		_ = g.statusText.PrintlnInverse("load busy: " + err.Error())
		return errors.New("load busy: " + err.Error())
	}

	_ = g.statusText.Println("CPUs: " + strconv.Itoa(runtime.NumCPU()))
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	g.totalRAM = strconv.Itoa(int(mem.HeapSys / 1024))
	_ = g.statusText.Println(strconv.Itoa(int(mem.HeapSys/1024)) + "k RAM, " + strconv.Itoa(int(mem.HeapIdle/1024)) + "k free")

	g.driver.LateInit(g.statusText)
	g.initMainMenu()

	_ = g.statusText.Print("Loading face")
	f, err = face.New(g)
	if err != nil {
		_ = g.statusText.PrintlnInverse(": " + err.Error())
		return errors.New("load face: " + err.Error())
	}

	_ = g.statusText.Println(".\nThe time is now")
	_ = g.statusText.Println(time.Now().Format(time.Stamp))
	_ = g.statusText.Println("Booted in " + time.Now().Sub(g.start).Round(100*time.Millisecond).String())
	_ = g.statusText.Println("Gotogen online.")

	// TODO load from settings storage; these is also defined in initMainMenu
	g.statusDownmixChannel = colorChannelRed
	g.statusDownmixCutoff = 0xA0
	g.statusFrameSkip = 0

	g.statusText.AutoFlush = false
	g.statusStateChange = time.Now()

	g.blink()
	g.init = true
	println("init complete in", time.Now().Sub(g.start).Round(100*time.Millisecond).String())
	return nil
}

// Run does not return. It attempts to run the main loop at the framerate specified in New.
func (g *Gotogen) Run() {
	for range time.Tick(g.frameTime) {
		err := g.RunTick()
		if err != nil {
			g.panic(err)
		}
	}
}

var s animation.Animation
var f *face.Anim

// RunTick runs a single iteration of the main loop.
func (g *Gotogen) RunTick() error {
	if !g.init {
		return errors.New("not initialized")
	}

	g.blinkerOff()
	g.tick++
	g.statusForceUpdate = false

	// busy states clear when we get back to the run loop
	if g.faceState == faceStateBusy {
		g.faceState = faceStateDefault
		f.Activate(g)
		g.activeAnim = f
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
		g.aX, g.aY, g.aZ = x, y, z
	}

	// TODO better way to framerate limit the status screen
	canRedrawStatus := g.statusDisplay.CanUpdateNow()
	if g.statusFrameSkip > 0 {
		canRedrawStatus = canRedrawStatus && uint8(g.tick)%g.statusFrameSkip == 0
	}
	// we always need to call this tho since the menu handling code is in here
	g.updateStatus(canRedrawStatus)

	cont := g.activeAnim.DrawFrame(g, g.tick)
	if !cont {
		g.faceState = faceStateDefault
		g.statusForceUpdate = true
		f.Activate(g)
		g.activeAnim = f
	}

	err := g.faceDisplay.Display()
	if err != nil {
		g.panic(err)
	}

	if g.statusState != statusStateBlank && canRedrawStatus {
		err = g.statusText.Display()
		if err != nil {
			g.panic(err)
		}
	}

	g.blinkerOn()
	return nil
}

func (g *Gotogen) drawIdleStatus() {
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	// TODO switch which line this is on every minute or so for burn-in protection
	_ = g.statusText.SetLine(0, time.Now().Format("03:04"), " ", strconv.Itoa(int(g.lastFPS)), "Hz ", strconv.Itoa(int(mem.HeapIdle/1024)), "k/", g.totalRAM, "k")
	// TODO temp hack
	_ = g.statusText.SetLine(1, strconv.Itoa(int(g.boopDist)), " ", strconv.Itoa(int(g.aX)), " ", strconv.Itoa(int(g.aY)), " ", strconv.Itoa(int(g.aZ)))

	_ = g.statusText.SetLine(3, g.driver.StatusLine())

	// println(time.Now().Format("03:04"), g.lastFPS, "Hz", mem.HeapIdle/1024, "k/", g.totalRAM)
}

func (g *Gotogen) updateStatus(updateIdleStatus bool) {
	switch g.statusState {
	case statusStateBoot:
		if time.Now().After(g.statusStateChange.Add(menuTimeout)) {
			g.changeStatusState(statusStateIdle)
			break
		}
		// any button press clears the boot log
		if g.driver.PressedButton() != MenuButtonNone {
			g.changeStatusState(statusStateIdle)
		}
	case statusStateIdle:
		but := g.driver.PressedButton()
		switch but {
		case MenuButtonBack:
			if g.faceState != faceStateDefault {
				g.faceState = faceStateDefault
				g.statusForceUpdate = true
				f.Activate(g)
				g.activeAnim = f
			}
		case MenuButtonMenu:
			g.changeStatusState(statusStateMenu)
		default:
			if updateIdleStatus {
				g.drawIdleStatus()
			}
		}
	case statusStateMenu:
		if time.Now().After(g.statusStateChange.Add(menuTimeout)) {
			g.changeStatusState(statusStateIdle)
			break
		}

		switch g.driver.PressedButton() {
		case MenuButtonBack:
			g.statusStateChange = time.Now()
			if g.activeMenu.Prev() == nil {
				// at top level menu
				g.changeStatusState(statusStateIdle)
			} else {
				m := g.activeMenu
				g.activeMenu = g.activeMenu.Prev()
				m.SetPrev(nil)
				g.activeMenu.Render(g.statusText)
			}
		case MenuButtonMenu:
			g.statusStateChange = time.Now()
			switch active := g.activeMenu.(type) {
			case *Menu:
				// in case a menu is empty for some reason
				if len(active.Items) == 0 || int(active.selected) > len(active.Items) {
					break
				}
				switch item := active.Items[active.selected].(type) {
				case *Menu:
					item.prev, g.activeMenu = g.activeMenu, item
					item.Render(g.statusText)
				case *ActionItem:
					item.Invoke()
				case *SettingItem:
					item.prev, g.activeMenu = g.activeMenu, item
					item.selected = item.Active
					_, h := g.statusText.Size()
					if item.selected > item.top+uint8(h)-2 {
						// TODO avoid empty lines at the bottom?
						item.top = item.selected
					}
					item.Render(g.statusText)
				}
			case *SettingItem:
				active.Active = active.selected
				active.Apply(active.selected)
				g.activeMenu, active.prev = active.prev, nil
				g.activeMenu.Render(g.statusText)
			}
		case MenuButtonUp:
			g.statusStateChange = time.Now()
			if g.activeMenu.Selected() > 0 {
				g.activeMenu.SetSelected(g.activeMenu.Selected() - 1)
			}
			if g.activeMenu.Selected() < g.activeMenu.Top() {
				g.activeMenu.SetTop(g.activeMenu.Selected())
			}
			g.activeMenu.Render(g.statusText)
		case MenuButtonDown:
			g.statusStateChange = time.Now()
			g.activeMenu.SetSelected(g.activeMenu.Selected() + 1)
			if g.activeMenu.Selected() > g.activeMenu.Len()-1 {
				g.activeMenu.SetSelected(g.activeMenu.Len() - 1)
			}
			_, h := g.statusText.Size()
			if g.activeMenu.Selected() > g.activeMenu.Top()+uint8(h)-2 {
				g.activeMenu.SetTop(g.activeMenu.Top() + 1)
			}
			g.activeMenu.Render(g.statusText)
		}
	case statusStateBlank:
		if g.driver.PressedButton() != MenuButtonNone {
			g.changeStatusState(statusStateIdle)
		}
	}
}

func (g *Gotogen) clearStatusScreen() {
	// clear text buffer
	g.statusText.Clear()
	// but make sure we clear the *entire* screen, including pixels outside the coverage of the text buffer
	w, h := g.statusDisplay.Size()
	for x := int16(0); x < w; x++ {
		for y := int16(0); y < h; y++ {
			g.statusDisplay.SetPixel(x, y, color.RGBA{})
		}
	}
	_ = g.statusDisplay.Display()
}

func (g *Gotogen) changeStatusState(state statusState) {
	println("changing to status state", state.String())
	g.activeMenu = nil
	g.statusState = state
	g.statusStateChange = time.Now()
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
		g.rootMenu.Render(g.statusText)
	}
}

func (g *Gotogen) startAnimation(a animation.Animation) {
	g.faceState = faceStateAnimation
	a.Activate(g)
	g.activeAnim = a
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
	g.blinkerOn()
	time.Sleep(100 * time.Millisecond)
	g.blinkerOff()
	time.Sleep(100 * time.Millisecond)
}

func (g *Gotogen) blinkerOn() {
	if g.blinker != nil {
		g.blinker.High()
	}
}

func (g *Gotogen) blinkerOff() {
	if g.blinker != nil {
		g.blinker.Low()
	}
}

func (g *Gotogen) newAnimation(file string, f func(string) (animation.Animation, error)) {
	a, err := f(file)
	if err != nil {
		g.panic(err)
	}
	g.startAnimation(a)
	// TODO exit the menu?
}

func (g *Gotogen) initMainMenu() {
	imgs, err := media.Enumerate(media.TypeFull)
	if err != nil {
		g.panic("enumerating images for animations: " + err.Error())
	}
	var anims []Item
	for _, i := range imgs {
		f := i
		anims = append(anims, &Menu{
			Name: i,
			Items: []Item{
				&ActionItem{
					Name:   "Static",
					Invoke: func() { g.newAnimation(f, static.New) },
				},
				&ActionItem{
					Name:   "Slide",
					Invoke: func() { g.newAnimation(f, slide.New) },
				},
				&ActionItem{
					Name:   "Peek",
					Invoke: func() { g.newAnimation(f, peek.New) },
				},
			},
		})
	}

	g.rootMenu = Menu{
		Name: "GOTOGEN MENU",
		Items: []Item{
			// This must be first, and will be filled in by the driver's menu items every time the menu is displayed.
			&Menu{
				Name: "Hardware Settings",
			},
			&Menu{
				Name:  "Full-screen anims.",
				Items: anims,
			},
			&Menu{
				Name: "Internal screen",
				Items: []Item{
					&ActionItem{
						Name:   "Blank screen",
						Invoke: func() { g.changeStatusState(statusStateBlank) },
					},
					&SettingItem{
						Name:    "Frame skip",
						Options: []string{"0", "1", "2", "4", "8", "16"},
						Active:  0, // TODO load from setting storage
						Apply:   g.setStatusFrameSkip,
					},
					&SettingItem{
						Name:    "Face dupl. color",
						Options: []string{"full", "red", "green", "blue"},
						Active:  1,
						Apply:   g.setStatusDuplicateColor,
					},
					&SettingItem{
						Name:    "Face dupl. cutoff",
						Options: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "A", "B", "D", "E", "F"},
						Active:  9,
						Apply:   g.setStatusDuplicateCutoff,
					},
				},
			},
		},
	}
}

func (g *Gotogen) setStatusDuplicateCutoff(selected uint8) {
	g.statusDownmixCutoff = (selected + 1) << 4
}

func (g *Gotogen) setStatusDuplicateColor(selected uint8) {
	g.statusDownmixChannel = colorChannel(selected)
}

func (g *Gotogen) setStatusFrameSkip(selected uint8) {
	if selected == 0 {
		g.statusFrameSkip = 0
	} else {
		g.statusFrameSkip = 1 << (selected - 1)
	}
}

func (g *Gotogen) Busy(f func(buffer *textbuf.Buffer)) {
	g.statusText.AutoFlush = true
	g.statusText.Clear()

	err := g.busy()
	if err != nil {
		print("unable to load busy", err)
		_ = g.statusText.PrintlnInverse("loading busy: " + err.Error())
	}
	f(g.statusText)

	s := time.Now()
	for time.Now().Before(s.Add(5 * time.Second)) {
		if g.driver.PressedButton() != MenuButtonNone {
			break
		}
	}

	g.statusText.AutoFlush = false
	g.changeStatusState(statusStateIdle)
}

func (g *Gotogen) busy() error {
	g.faceState = faceStateBusy

	busy, err := static.New("wait")
	if err != nil {
		return errors.New("load busy: " + err.Error())
	}
	busy.Activate(g.faceMirror)
	_ = g.faceDisplay.Display()
	g.activeAnim = busy

	return nil
}

func (g *Gotogen) Size() (x, y int16) {
	return g.faceMirror.Size()
}

func (g *Gotogen) Display() error {
	// nothing to do here, refreshing the real displays is managed elsewhere
	return nil
}

func (g *Gotogen) SetPixel(x, y int16, c color.RGBA) {
	g.faceMirror.SetPixel(x, y, c)
	if g.statusForceUpdate || (g.statusState == statusStateIdle && (g.statusFrameSkip == 0 || uint8(g.tick)%g.statusFrameSkip == 0 && g.statusDisplay.CanUpdateNow())) {
		switch g.statusDownmixChannel {
		case colorChannelRed:
			if c.R < g.statusDownmixCutoff {
				c.R = 0
			} else {
				c.R = 0xFF
			}
			c.G = 0
			c.B = 0
		case colorChannelGreen:
			if c.G < g.statusDownmixCutoff {
				c.G = 0
			} else {
				c.G = 0xFF
			}
			c.R = 0
			c.B = 0
		case colorChannelBlue:
			if c.B < g.statusDownmixCutoff {
				c.B = 0
			} else {
				c.B = 0xFF
			}
			c.R = 0
			c.G = 0
		}
		// TODO remove hardcoded offset
		g.statusMirror.SetPixel(x, y+32, c)
	}
}

func (g *Gotogen) Talking() bool {
	return g.driver.Talking()
}
