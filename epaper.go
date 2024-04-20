package epaper

import (
	"image"
	"image/color"
	"log"
	"os"
	"time"

	rpio "github.com/stianeikeland/go-rpio/v4"
)

type Epaper struct {
	rstPin  rpio.Pin
	dcPin   rpio.Pin
	csPin   rpio.Pin
	busyPin rpio.Pin
	device  *os.File

	displayWidth  int
	displayHeight int

	lastFrame *image.Gray
}

func New(device *os.File, displayWidth, displayHeight int) *Epaper {
	e := &Epaper{
		rstPin:        rpio.Pin(17),
		dcPin:         rpio.Pin(25),
		csPin:         rpio.Pin(8),
		busyPin:       rpio.Pin(24),
		device:        device,
		displayWidth:  displayWidth,
		displayHeight: displayHeight,
	}
	// initialize pins
	e.rstPin.Output()
	e.dcPin.Output()
	e.csPin.Output()
	e.busyPin.Input()
	return e
}

func (e *Epaper) init() {
	e.reset()
	e.sendCommand(powerSetting)
	e.sendData(0x03) // VDS_EN, VDG_EN
	e.sendData(0x00) // VCOM_HV, VGHL_LV[1], VGHL_LV[0]
	e.sendData(0x2b) // VDH
	e.sendData(0x2b) // VDL
	e.sendData(0x09) // VDHR
	e.sendCommand(boosterSoftStart)
	e.sendData(0x07)
	e.sendData(0x07)
	e.sendData(0x17)
	// Power optimization
	e.sendCommand(0xF8)
	e.sendData(0x60)
	e.sendData(0xA5)
	// Power optimization
	e.sendCommand(0xF8)
	e.sendData(0x89)
	e.sendData(0xA5)
	// Power optimization
	e.sendCommand(0xF8)
	e.sendData(0x90)
	e.sendData(0x00)
	// Power optimization
	e.sendCommand(0xF8)
	e.sendData(0x93)
	e.sendData(0x2A)
	// Power optimization
	e.sendCommand(0xF8)
	e.sendData(0xA0)
	e.sendData(0xA5)
	// Power optimization
	e.sendCommand(0xF8)
	e.sendData(0xA1)
	e.sendData(0x00)
	// Power optimization
	e.sendCommand(0xF8)
	e.sendData(0x73)
	e.sendData(0x41)
	e.sendCommand(partialDisplayRefresh)
	e.sendData(0x00)
	e.sendCommand(powerOn)

	e.waitUntilIdle()
	e.sendCommand(panelSetting)
	e.sendData(0xAF) // KW-BF   KWR-AF    BWROTP 0f
	e.sendCommand(pllControl)
	e.sendData(0x3A) // 3A 100HZ   29 150Hz 39 200HZ    31 171HZ
	e.sendCommand(vcmDcSettingRegister)
	e.sendData(0x12)
	time.Sleep(2 * time.Millisecond)
	e.setLut()
	// EPD hardware init end
}

func getImageBuffer(img image.Image) []byte {
	img = flipImage(img)

	bounds := img.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y

	bufferLength := width * height / 8
	buf := make([]byte, bufferLength)
	// update the buffer with white values
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			grayColor := img.At(x, y).(color.Gray)
			if grayColor.Y > 0 {
				buf[(y+x*height)/8] |= (0x80 >> (uint(y) % uint(8)))
			}
		}
	}
	return buf
}

// TODO: why does the image need to be flipped?
func flipImage(img image.Image) image.Image {
	b := img.Bounds()
	flippedImg := image.NewGray(b)
	for y := 0; y < b.Max.Y; y++ {
		for x := 0; x < b.Max.X; x++ {
			flippedImg.Set(b.Max.X-x, y, img.At(x, y))
		}
	}
	img = flippedImg
	return img
}

func (e *Epaper) reset() {
	e.rstPin.Low()
	time.Sleep(200 * time.Millisecond)
	e.rstPin.High()
	time.Sleep(200 * time.Millisecond)
}

func (e *Epaper) sendCommand(b byte) {
	e.dcPin.Low()
	_, err := e.device.Write([]byte{b})
	if err != nil {
		log.Fatalf("failed to write command to device: %s", err)
	}
}

func (e *Epaper) sendData(data ...byte) {
	e.dcPin.High()
	for i := 0; i < len(data); i++ {
		// b := data[i]
		_, err := e.device.Write(data[i : i+1])
		if err != nil {
			log.Fatalf("failed to write data byte to device: %s", err)
		}
	}
}

func (e *Epaper) waitUntilIdle() {
	for {
		if e.busyPin.Read() == rpio.High {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (e *Epaper) setLut() {
	e.sendCommand(lutForVcom) // vcom
	e.sendData(lutVcomDC...)

	e.sendCommand(lutWhiteToWhite) // ww --
	e.sendData(lutWw...)

	e.sendCommand(lutBlackToWhite) // bw r
	e.sendData(lutBw...)

	e.sendCommand(lutWhiteToBlack) // wb w
	e.sendData(lutBb...)

	e.sendCommand(lutBlackToBlack) // bb b
	e.sendData(lutWb...)
}

func (e *Epaper) WriteImage(img *image.Gray) {
	e.lastFrame = img

	e.init()
	buf := getImageBuffer(img)

	e.sendCommand(dataStartTransmission1)
	time.Sleep(2 * time.Millisecond)
	for i := 0; i < len(buf); i++ {
		e.sendData(0xFF)
	}
	time.Sleep(2 * time.Millisecond)

	e.sendCommand(dataStartTransmission2)
	time.Sleep(2 * time.Millisecond)
	e.sendData(buf...)
	time.Sleep(2 * time.Millisecond)

	e.sendCommand(displayRefresh)
	e.waitUntilIdle()
}
