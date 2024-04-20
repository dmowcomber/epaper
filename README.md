# epaper

epaper is an example of how to display an image to a Waveshare 2.7 inch epaper display hat with the Raspberry Pi 3 B+ written in Golang.

## running the demo

#### Setup
[Setup SPI](https://www.raspberrypi.org/documentation/hardware/raspberrypi/spi/README.md) by uncommenting `dtparam=spi=on` in `/boot/config.txt`. Then reboot `sudo reboot`.

#### Run
Run an epaper example with Golang
```bash
go run -mod=vendor ./examples/image
```

Run epaper with Docker
```
docker build . -t dmowcomber/epaper
docker run --rm -v /dev/mem:/dev/mem --device /dev/gpiomem --device /dev/spidev0.0 dmowcomber/epaper
```

<img align="center" src="readme.jpg" width="50%" height="50%">
