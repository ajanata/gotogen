package main

import (
	"machine"
	"runtime"
	"time"
	"tinygo.org/x/drivers"

	"tinygo.org/x/drivers/ssd1306"

	"github.com/ajanata/gotogen"
)

func main() {
	runtime.AdjustTimeOffset(int64(1665706022 * time.Second))

	machine.LED.Configure(machine.PinConfig{Mode: machine.PinOutput})
	blink()

	machine.I2C0.Configure(machine.I2CConfig{
		SCL: machine.I2C1_SCL_PIN,
		SDA: machine.I2C1_SDA_PIN,
	})

	dev := ssd1306.NewI2C(machine.I2C0)
	dev.Configure(ssd1306.Config{Width: 128, Height: 64, Address: 0x3D, VccState: ssd1306.SWITCHCAPVCC})
	dev.ClearBuffer()
	dev.ClearDisplay()

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
	machine.LED.High()
	time.Sleep(500 * time.Millisecond)
	machine.LED.Low()
	time.Sleep(500 * time.Millisecond)
}

func earlyPanic() {
	for {
		blink()
	}
}
