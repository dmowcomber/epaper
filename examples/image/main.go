package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	_ "image/jpeg"
	"log"
	"os"

	"github.com/MaxHalford/halfgone"
	epaper "github.com/dmowcomber/go-epaper-demo"
	rpio "github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/sys/unix"
)

//go:embed image.jpg
var imgData []byte

func main() {
	// display resolution
	const displayWidth = 264
	const displayHeight = 176
	// device path
	const devicePath = "/dev/spidev0.0"

	err := rpio.Open()
	if err != nil {
		log.Fatalf("unable to open pin: %#v", err)
	}
	defer rpio.Close()
	device, err := os.OpenFile(devicePath, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0666)
	if err != nil {
		log.Fatalf("unable to open device: %#v", err)
	}
	defer device.Close()
	epaper := epaper.New(device, displayWidth, displayHeight)

	img := loadImage()
	img = convertImageToGray(img)
	// draw result image
	epaper.WriteImage(img)
}

func convertImageToGray(img image.Image) *image.Gray {
	b := img.Bounds()
	grayImg := image.NewGray(b)
	for y := 0; y < b.Max.Y; y++ {
		for x := 0; x < b.Max.X; x++ {
			grayImg.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}
	return grayImg
}

func loadImage() *image.Gray {
	imgBuffer := bytes.NewBuffer(imgData)

	img, _, err := image.Decode(imgBuffer)
	if err != nil {
		log.Fatal(err)
	}

	grayImage := convertImageToGray(img)
	ditheredImg := halfgone.ThresholdDitherer{Threshold: 127}.Apply(grayImage)
	return ditheredImg
}
