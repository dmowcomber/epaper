package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/dmowcomber/epaper"
	"github.com/fogleman/gg"
	"github.com/robfig/cron"
	rpio "github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/sys/unix"
)

// TODO: when IP updated, force an update to the screen

// display resolution
const (
	displayWidth  = 264
	displayHeight = 176
	fontSize      = 26.0
)

func main() {
	// device path
	const devicePath = "/dev/spidev0.0"

	err := rpio.Open()
	if err != nil {
		log.Fatalf("unable to open pin: %#v", err)
	}
	defer rpio.Close()
	device, err := openDevice(devicePath)
	if err != nil {
		log.Fatalf("unable to open device: %#v", err)
	}
	defer device.Close()
	epaper := epaper.New(device, displayWidth, displayHeight)

	ex := ipDemo{epaper: epaper}
	ex.run()

	c := cron.New()
	c.AddFunc("1 * * * * *", ex.run)
	go c.Start()
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

func openDevice(devicePath string) (*os.File, error) {
	return os.OpenFile(devicePath, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0666)
}

type ipDemo struct {
	epaper *epaper.Epaper
}

func (ex *ipDemo) run() {
	now := time.Now()
	progressBarUntil := now.Truncate(60 * time.Second).Add(-1 * time.Second).Add(1 * time.Minute)

	img := image.NewGray(image.Rect(0, 0, displayWidth, displayHeight))

	ip := getOutboundIP()
	text := fmt.Sprintf("out: %s\n", ip.String())

	ifaceIPs := getInterfaceIPs()
	for _, ifaceIP := range ifaceIPs {
		text = fmt.Sprintf("%s%s: %s\n", text, ifaceIP.ifaceName, ifaceIP.ipAddress)
	}
	textImg := addTextToImage(img, color.White, text, fontSize, 5, -10)

	img = convertImageToGray(textImg)
	ex.epaper.WriteImage(img)

	/*
		progress indicator - draw a flashing square so we know the program is still running
	*/
	width := 16
	height := 16
	colorIsBlack := true

	blackSquareImg := image.NewGray(image.Rect(0, 0, width, height))
	draw.Draw(blackSquareImg, blackSquareImg.Rect, &image.Uniform{color.Black}, image.Point{}, draw.Src)
	progressImage := blackSquareImg

	whiteSquareImg := image.NewGray(image.Rect(0, 0, width, height))
	draw.Draw(whiteSquareImg, whiteSquareImg.Rect, &image.Uniform{color.White}, image.Point{}, draw.Src)

	log.Printf("time.Until(progressBarUntil): %s", time.Until(progressBarUntil))
	for i := 0; time.Until(progressBarUntil) > 0; i++ {
		log.Printf("time.Until(progressBarUntil): %s", time.Until(progressBarUntil))
		if i%3 == 0 {
			if colorIsBlack {
				colorIsBlack = false
				progressImage = whiteSquareImg
			} else {
				colorIsBlack = true
				progressImage = blackSquareImg
			}
		}
		ex.epaper.WriteImageQuick(progressImage, 0, 0)
	}
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

func generateFont(fontSize float64) (font.Face, error) {
	fontData := goregular.TTF
	tt, err := opentype.Parse(fontData)
	if err != nil {
		log.Fatal(err)
	}

	const (
		dpi = 72
	)

	return opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
}

func addTextToImage(img image.Image, clr color.Color, text string, fontSize, xOffset, yOffset float64) image.Image {
	dc := gg.NewContextForImage(img)
	dc.SetRGB(1, 1, 1)
	fnt, err := generateFont(fontSize)
	if err != nil {
		// TODO: return error
		log.Fatalf("failed to generate font: %s", err)
	}
	dc.SetFontFace(fnt)
	dc.SetColor(clr)
	// dc.DrawStringWrapped(text, float64(b.Max.X)/2, float64(b.Max.Y)/2, 0.5, 0.5, displayWidth, 1, gg.AlignLeft)
	dc.DrawStringWrapped(text, 0+xOffset, 0+yOffset, 0, 0, displayWidth, 1, gg.AlignLeft)
	return dc.Image()
}

// getOutboundIP returns the IP that's talking to the internet
// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func getOutboundIP() net.IP {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 1*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

type ifaceIP struct {
	ifaceName string
	ipAddress string
}

// getInterfaceIPs gets v4 IPs for each network interface
// ignoring any loopback IPs
// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func getInterfaceIPs() []ifaceIP {
	result := make([]ifaceIP, 0)

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			log.Fatal(err)
		}
		for _, addr := range addrs {
			if addr == nil {
				log.Printf("NIL! %s", iface.Name)
			}
			var currentIP net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				currentIP = v.IP.To4()
			case *net.IPAddr:
				currentIP = v.IP.To4()
			}
			log.Printf("%s: %s, %#v %d", iface.Name, currentIP.String(), currentIP, len(currentIP))
			if currentIP != nil && len(currentIP) != 0 && !currentIP.IsLoopback() {
				log.Printf("%s: appending %s, %#v %d", iface.Name, currentIP.String(), currentIP, len(currentIP))
				result = append(result, ifaceIP{ifaceName: iface.Name, ipAddress: currentIP.String()})
			}
		}
	}
	return result
}
