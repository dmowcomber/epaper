package main

import (
	"image"
	"image/color"
	_ "image/jpeg"
	"log"
	"os"
	"time"

	"github.com/MaxHalford/halfgone"
	rpio "github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/sys/unix"
)

const (
	// display resolution
	epdWidth  = 176
	epdHeight = 264

	// device
	devicePath = "/dev/spidev0.0"
)

var (
	// pins
	rstPin  rpio.Pin
	dcPin   rpio.Pin
	csPin   rpio.Pin
	busyPin rpio.Pin
)

func init() {
	err := rpio.Open()
	if err != nil {
		log.Fatalf("unable to open pin: %#v", err)
	}

	// initialize pins
	rstPin = rpio.Pin(17)
	dcPin = rpio.Pin(25)
	csPin = rpio.Pin(8)
	busyPin = rpio.Pin(24)

	// rstPin.Mode(mode)

	rstPin.Output()
	dcPin.Output()
	csPin.Output()
	busyPin.Input()
}

func main() {
	device, err := openDev()
	if err != nil {
		log.Fatalf("unable to open device: %#v", err)
	}

	reset()
	sendCommand(device, powerSetting)
	sendData(device, 0x03) // VDS_EN, VDG_EN
	sendData(device, 0x00) // VCOM_HV, VGHL_LV[1], VGHL_LV[0]
	sendData(device, 0x2b) // VDH
	sendData(device, 0x2b) // VDL
	sendData(device, 0x09) // VDHR
	sendCommand(device, boosterSoftStart)
	sendData(device, 0x07)
	sendData(device, 0x07)
	sendData(device, 0x17)
	// Power optimization
	sendCommand(device, 0xF8)
	sendData(device, 0x60)
	sendData(device, 0xA5)
	// Power optimization
	sendCommand(device, 0xF8)
	sendData(device, 0x89)
	sendData(device, 0xA5)
	// Power optimization
	sendCommand(device, 0xF8)
	sendData(device, 0x90)
	sendData(device, 0x00)
	// Power optimization
	sendCommand(device, 0xF8)
	sendData(device, 0x93)
	sendData(device, 0x2A)
	// Power optimization
	sendCommand(device, 0xF8)
	sendData(device, 0xA0)
	sendData(device, 0xA5)
	// Power optimization
	sendCommand(device, 0xF8)
	sendData(device, 0xA1)
	sendData(device, 0x00)
	// Power optimization
	sendCommand(device, 0xF8)
	sendData(device, 0x73)
	sendData(device, 0x41)
	sendCommand(device, partialDisplayRefresh)
	sendData(device, 0x00)
	sendCommand(device, powerOn)

	waitUntilIdle()
	sendCommand(device, panelSetting)
	sendData(device, 0xAF) // KW-BF   KWR-AF    BWROTP 0f
	sendCommand(device, pllControl)
	sendData(device, 0x3A) // 3A 100HZ   29 150Hz 39 200HZ    31 171HZ
	sendCommand(device, vcmDcSettingRegister)
	sendData(device, 0x12)
	time.Sleep(2 * time.Millisecond)
	setLut(device)
	// EPD hardware init end

	log.Println("opening image")
	imageFile, err := os.Open("image.jpg")
	if err != nil {
		log.Fatal(err)
	}
	defer imageFile.Close()

	log.Println("decode")
	img, _, err := image.Decode(imageFile)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("convert image to gray")
	b := img.Bounds()
	grayImage := image.NewGray(b)
	for y := 0; y < b.Max.Y; y++ {
		for x := 0; x < b.Max.X; x++ {
			grayImage.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}

	log.Println("new img")

	log.Println("dither")
	// ditheredImage := halfgone.SierraLiteDitherer{}.Apply(grayImage)
	ditheredImage := halfgone.ThresholdDitherer{Threshold: 127}.Apply(grayImage)

	log.Println("disp")

	// initialize buffer with all zeros (black)
	bufferLength := epdWidth * epdHeight / 8
	buf := make([]byte, bufferLength)
	for i := 0; i < bufferLength; i++ {
		buf[i] = 0x00
	}

	// update the buffer with white values
	for y := 0; y < epdHeight; y++ {
		for x := 0; x < epdWidth; x++ {
			grayColor := ditheredImage.At(y, x).(color.Gray)
			// if grayColor.Y == 0 {
			if grayColor.Y > 0 {
				buf[(x+y*epdWidth)/8] |= (0x80 >> (uint(x) % uint(8)))
			}
		}
	}

	displayFrame(device, buf)
}

func openDev() (*os.File, error) {
	return os.OpenFile(devicePath, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0666)
}

func reset() {
	rstPin.Low()
	time.Sleep(200 * time.Millisecond)
	rstPin.High()
	time.Sleep(200 * time.Millisecond)
}

func sendCommand(device *os.File, b byte) {
	dcPin.Low()
	_, err := device.Write([]byte{b})
	if err != nil {
		log.Fatalf("failed to write command to device: %#v", err)
	}
}

func sendData(device *os.File, b byte) {
	dcPin.High()
	_, err := device.Write([]byte{b})
	if err != nil {
		log.Fatalf("failed to write data to device: %#v", err)
	}
}

func waitUntilIdle() {
	for {
		if busyPin.Read() == rpio.High {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func setLut(device *os.File) {
	sendCommand(device, lutForVcom) // vcom
	for count := 0; count < 44; count++ {
		sendData(device, lutVcomDC[count])
	}
	sendCommand(device, lutWhiteToWhite) // ww --
	for count := 0; count < 42; count++ {
		sendData(device, lutWw[count])
	}
	sendCommand(device, lutBlackToWhite) // bw r
	for count := 0; count < 42; count++ {
		sendData(device, lutBw[count])
	}
	sendCommand(device, lutWhiteToBlack) // wb w
	for count := 0; count < 42; count++ {
		sendData(device, lutBb[count])
	}
	sendCommand(device, lutBlackToBlack) // bb b
	for count := 0; count < 42; count++ {
		sendData(device, lutWb[count])
	}
}

func displayFrame(device *os.File, b []byte) {
	size := len(b)

	sendCommand(device, dataStartTransmission1)

	time.Sleep(2 * time.Millisecond)

	for i := 0; i < size; i++ {
		sendData(device, 0xFF)
	}
	time.Sleep(2 * time.Millisecond)

	sendCommand(device, dataStartTransmission2)
	time.Sleep(2 * time.Millisecond)

	for i := 0; i < size; i++ {
		sendData(device, b[i])
	}
	time.Sleep(2 * time.Millisecond)

	sendCommand(device, displayRefresh)
	waitUntilIdle()
}
