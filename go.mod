module github.com/ajanata/gotogen

go 1.19

// fixes compile error for apds9960
replace tinygo.org/x/drivers => github.com/ajanata/tinygo-drivers v0.0.0-20221010064956-016cdce8a129

require (
	github.com/ajanata/textbuf v0.0.2
	tinygo.org/x/drivers v0.23.0
)

require github.com/ajanata/oled_font v1.2.0 // indirect
