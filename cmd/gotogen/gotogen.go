package main

import (
	"machine"
	"time"
	"tinygo.org/x/drivers"

	"tinygo.org/x/drivers/ssd1306"

	"github.com/ajanata/gotogen"
)

func main() {
	blink()
	machine.I2C0.Configure(machine.I2CConfig{
		SCL:       machine.I2C0_SCL_PIN,
		SDA:       machine.I2C0_SDA_PIN,
		Frequency: 2 * machine.MHz,
	})
	blink()

	dev := ssd1306.NewI2C(machine.I2C0)
	dev.Configure(ssd1306.Config{Width: 128, Height: 64, Address: 0x3D, VccState: ssd1306.SWITCHCAPVCC})
	blink()
	dev.ClearBuffer()
	dev.ClearDisplay()
	blink()

	g, err := gotogen.New(60, nil, &dev, machine.LED, func() (faceDisplay drivers.Displayer, menuInput gotogen.MenuInput, boopSensor gotogen.BoopSensor, err error) {
		return nil, nil, nil, nil
	})
	if err != nil {
		earlyPanic()
	}
	err = g.Init()
	if err != nil {
		earlyPanic()
	}

	for {
		time.Sleep(time.Hour)
	}
}

func blink() {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led.High()
	time.Sleep(100 * time.Millisecond)
	led.Low()
	time.Sleep(100 * time.Millisecond)
}

func earlyPanic() {
	for {
		blink()
	}
}
