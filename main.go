package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kylelemons/gousb/usb"
)

const (
	QQ2440_VENDOR_ID  = 0x5345
	QQ2440_PRODUCT_ID = 0x1234
	FS2410_VENDOR_ID  = 0x5345
	FS2410_PRODUCT_ID = 0x1234
	EZ6410_VENDOR_ID  = 0x04e8
	EZ6410_PRODUCT_ID = 0x1234

	EZ6410_RAM_BASE = 0x50200000
	FS2410_RAM_BASE = 0x30200000
	RAM_BASE        = EZ6410_RAM_BASE
	VENDOR_ID       = EZ6410_VENDOR_ID
	PRODUCT_ID      = EZ6410_PRODUCT_ID
)

var usbDebug int

func init() {
	flag.IntVar(&usbDebug, "usbdebug", 0, "libusb debug level (0-3)")
}

func flash(buf []byte) {
	ctx := usb.NewContext()
	defer ctx.Close()
	ctx.Debug(usbDebug)
	devices, err := ctx.ListDevices(func(desc *usb.Descriptor) bool {
		return desc.Vendor == VENDOR_ID && desc.Product == PRODUCT_ID
	})

	if err != nil {
		log.Fatal(err)
	}

	for _, d := range devices {
		fmt.Printf("%s:%s\n", d.Descriptor.Vendor, d.Descriptor.Product)
		defer d.Close()
	}
	if len(devices) == 0 {
		log.Println("Could not find device")
		return
	}

	d := devices[0]
	d.WriteTimeout = 3 * time.Second

	//Config 1, Interface 0, setup 0, Endpoint 2 as deduced from lsusb
	e, err := d.OpenEndpoint(1, 0, 0, 2)

	if err != nil {
		log.Fatal(err)
	}

	maxSize := int(e.Info().MaxPacketSize)

	for i := 0; i < len(buf); {
		top := i + maxSize
		if top > len(buf) {
			top = len(buf)
		}
		s, err := e.Write(buf[i:top])
		if err != nil {
			log.Printf("Written %d of %d bytes\n", i+s, len(buf))
			log.Fatal(err)
		}
		i += s
	}
}

//checksum computes a simple 16bit checksum on the given buffer
func checksum(buf []byte) uint16 {
	var csum uint16
	for _, v := range buf {
		csum += uint16(v)
	}
	return csum
}

//prepareWriteBuffer creates a buffer with the file contents and some metadata
func prepareWriteBuf(filename string) []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, uint32(RAM_BASE))

	fi, err := os.Stat(filename)
	if err != nil {
		log.Fatal(err)
	}
	binary.Write(buf, binary.LittleEndian, uint32(fi.Size()))

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	buf.ReadFrom(f)
	if err != nil {
		log.Fatal(err)
	}

	csum := checksum(buf.Bytes()[8:])
	binary.Write(buf, binary.LittleEndian, csum)
	log.Printf("%X", csum)
	return buf.Bytes()
}

func main() {
	flag.Parse()
	files := flag.Args()
	if len(files) == 0 {
		fmt.Println("Usage: dnw <file>")
		return
	}
	buf := prepareWriteBuf(files[0])
	flash(buf)
}
