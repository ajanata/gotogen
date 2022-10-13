package main

import (
	"fmt"
	"machine"
	"time"

	"github.com/ajanata/textbuf"
	"tinygo.org/x/drivers/apds9960"
	"tinygo.org/x/drivers/ssd1306"
)

func blink() {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led.High()
	time.Sleep(100 * time.Millisecond)
	led.Low()
	time.Sleep(100 * time.Millisecond)
}

func main() {
	blink()
	machine.I2C0.Configure(machine.I2CConfig{
		SCL: machine.I2C1_SCL_PIN,
		SDA: machine.I2C1_SDA_PIN,
	})
	blink()

	dev := ssd1306.NewI2C(machine.I2C0)
	dev.Configure(ssd1306.Config{Width: 128, Height: 64, Address: 0x3D, VccState: ssd1306.SWITCHCAPVCC})
	blink()

	dev.ClearBuffer()
	dev.ClearDisplay()
	blink()

	buf, err := textbuf.New(&dev, textbuf.FontSize6x8)
	if err != nil {
		for {
			blink()
		}
	}

	buf.Println("playground boot")

	prox := apds9960.New(machine.I2C0)
	prox.Configure(apds9960.Configuration{})
	prox.EnableProximity()

	buf.PrintlnInverse("inverse")
	w, h := buf.Size()
	buf.Println(fmt.Sprintf("w, h = %d, %d", w, h))
	buf.SetLineInverse(5, "more inverse")

	for {
		time.Sleep(time.Second)
		blink()
		buf.SetLine(7, fmt.Sprintf("prox: %d", prox.ReadProximity()))
	}
}
