package st7735

import (
	"encoding/binary"
	"errors"
	"image"
	"time"

	"periph.io/x/periph/conn"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/conn/spi"
)

const (
	width  = 162
	height = 132

	nOP           = 0x00
	softwareRst   = 0x01
	readID        = 0x04
	sleepIn       = 0x10
	sleepOut      = 0x11
	partialOn     = 0x12
	partialOff    = 0x13
	inverseOn     = 0x20
	inverseOff    = 0x21
	gammaCurve    = 0x26
	displayOff    = 0x28
	displayOn     = 0x29
	columnAddress = 0x2A
	rowAddress    = 0x2B
	memoryWrite   = 0x2C
	memoryRead    = 0x2E
	partialStart  = 0x30
	tearingOff    = 0x34
	tearingOn     = 0x35
	memoryDAC     = 0x36
	idleOff       = 0x38
	idleOn        = 0x39
	pixelFormat   = 0x3a

	//Frame control settings
	frameControl1 = 0xB1
	frameControl2 = 0xB2
	frameControl3 = 0xB3

	invControl = 0xB4

	//power control settings
	powerControl1 = 0xC0
	powerControl2 = 0xC1
	powerControl3 = 0xC2
	powerControl4 = 0xC3
	powerControl5 = 0xC4
	vmControl1    = 0xC5
	vmOffControl  = 0xC7

	//gamma control
	gammaControlPositive = 0xE0
	gammaControlNegative = 0xE1
)

type Model interface {
	OffsetX() byte
	OffsetXEnd() byte
	OffsetY() byte
	OffsetYEnd() byte
}

type ModelStandard struct {
}

func (mm ModelStandard) OffsetX() byte {
	return 0
}

func (mm ModelStandard) OffsetXEnd() byte {
	return width - 1
}

func (mm ModelStandard) OffsetY() byte {
	return 0
}

func (mm ModelStandard) OffsetYEnd() byte {
	return height - 1
}

type ModelMini struct {
}

func (mm ModelMini) OffsetY() byte {
	return (width - 160) / 2
}

func (mm ModelMini) OffsetYEnd() byte {
	return (160 + mm.OffsetY()) - 1
}

func (mm ModelMini) OffsetX() byte {
	return (height - 80) / 2
}

func (mm ModelMini) OffsetXEnd() byte {
	return (80 + mm.OffsetX()) - 1
}

type Command struct {
	Command byte
	Data    []byte
	Delay   time.Duration
}

// Dev is a handle to a ST7735.
type Dev struct {
	c conn.Conn

	//dc low when sending a command, high when sending data.
	dc gpio.PinOut
	//rst reset pin, active low.
	rst gpio.PinOut
	//cs chip select pin
	cs gpio.PinIn
	//maxTxSize the maximum size of byte in a transaction
	maxTxSize int

	Model Model
}

// New opens a handle to a ST7735 LCD.
func New(p spi.Port, dc gpio.PinOut, rst gpio.PinOut, cs gpio.PinIn, m Model) (*Dev, error) {
	c, err := p.Connect(4000*physic.KiloHertz, spi.Mode0, 8)

	if err != nil {
		return nil, errors.New("could not connect to device")
	}

	// Get the maxTxSize from the conn if it implements the conn.Limits interface,
	// otherwise use 4096 bytes.
	maxTxSize := 0
	if limits, ok := c.(conn.Limits); ok {
		maxTxSize = limits.MaxTxSize()
	}

	if maxTxSize == 0 {
		maxTxSize = 4096 // Use a conservative default.
	}

	d := &Dev{
		c:         c,
		dc:        dc,
		rst:       rst,
		cs:        cs,
		maxTxSize: maxTxSize,
		Model:     m,
	}

	if err = d.reset(); err != nil {
		return nil, err
	}

	var cmd []Command
	cmd = append(cmd, Command{
		Command: softwareRst,
		Delay:   time.Duration(50 * time.Millisecond),
	})
	cmd = append(cmd, Command{
		Command: sleepOut,
		Delay:   time.Duration(50 * time.Millisecond),
	})
	cmd = append(cmd, Command{
		Command: frameControl1,
		Data:    []byte{0x01, 0x2C, 0x2D},
	})
	cmd = append(cmd, Command{
		Command: frameControl2,
		Data:    []byte{0x01, 0x2C, 0x2D},
	})
	cmd = append(cmd, Command{
		Command: frameControl3,
		Data:    []byte{0x01, 0x2C, 0x2D, 0x01, 0x2C, 0x2D},
	})
	cmd = append(cmd, Command{
		Command: invControl,
		Data:    []byte{0x07},
	})
	cmd = append(cmd, Command{
		Command: powerControl1,
		Data:    []byte{0xA2, 0x02, 0x84},
	})
	cmd = append(cmd, Command{
		Command: powerControl2,
		Data:    []byte{0xC5},
	})
	cmd = append(cmd, Command{
		Command: powerControl3,
		Data:    []byte{0x0A, 0x00},
	})
	cmd = append(cmd, Command{
		Command: powerControl4,
		Data:    []byte{0x8A, 0x2A},
	})
	cmd = append(cmd, Command{
		Command: powerControl5,
		Data:    []byte{0x8A, 0xEE},
	})
	cmd = append(cmd, Command{
		Command: vmControl1,
		Data:    []byte{0x0E},
	})
	cmd = append(cmd, Command{
		Command: inverseOff,
		Data:    nil,
	})
	cmd = append(cmd, Command{
		Command: memoryDAC,
		Data:    []byte{0xC8},
	})
	cmd = append(cmd, Command{
		Command: pixelFormat,
		Data:    []byte{0x05},
	})
	cmd = append(cmd, Command{
		Command: columnAddress,
		Data:    []byte{columnAddress, 0x00, byte(d.Model.OffsetX()), 0x00, byte(d.Model.OffsetXEnd())},
	})
	cmd = append(cmd, Command{
		Command: rowAddress,
		Data:    []byte{rowAddress, 0x00, byte(d.Model.OffsetY()), 0x00, byte(d.Model.OffsetYEnd())},
	})
	cmd = append(cmd, Command{
		Command: gammaControlPositive,
		Data:    []byte{0x02, 0x1C, 0x07, 0x12, 0x37, 0x32, 0x29, 0x2D, 0x29, 0x25, 0x2B, 0x39, 0x00, 0x01, 0x03, 0x10},
	})
	cmd = append(cmd, Command{
		Command: gammaControlNegative,
		Data:    []byte{0x03, 0x1d, 0x07, 0x06, 0x2E, 0x2C, 0x29, 0x2D, 0x2E, 0x2E, 0x37, 0x3F, 0x00, 0x00, 0x02, 0x10},
	})
	cmd = append(cmd, Command{
		Command: partialOff,
		Data:    []byte{0x03, 0x1d, 0x07, 0x06, 0x2E, 0x2C, 0x29, 0x2D, 0x2E, 0x2E, 0x37, 0x3F, 0x00, 0x00, 0x02, 0x10},
		Delay:   time.Duration(10 * time.Millisecond),
	})
	cmd = append(cmd, Command{
		Command: displayOn,
		Data:    []byte{0x03, 0x1d, 0x07, 0x06, 0x2E, 0x2C, 0x29, 0x2D, 0x2E, 0x2E, 0x37, 0x3F, 0x00, 0x00, 0x02, 0x10},
		Delay:   time.Duration(100 * time.Millisecond),
	})

	for _, c := range cmd {
		if err := d.send(c); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *Dev) setWindowAddress(x0, x1, y0, y1 byte) error {
	column := Command{
		Command: columnAddress,
		Data: []byte{
			x0 >> 8,
			x0,
			x1 >> 8,
			x1,
		},
	}

	if err := d.send(column); err != nil {
		return err
	}

	row := Command{
		Command: rowAddress,
		Data: []byte{
			y0 >> 8,
			y0,
			y1 >> 8,
			y1,
		},
	}

	if err := d.send(row); err != nil {
		return err
	}

	if err := d.sendCommand([]byte{memoryWrite}); err != nil {
		return err
	}

	return nil
}

func (d *Dev) SetImage(img image.Image) error {
	if err := d.setWindowAddress(d.Model.OffsetX(), d.Model.OffsetXEnd(), d.Model.OffsetY(), d.Model.OffsetYEnd()); err != nil {
		return err
	}

	if err := d.sendData(toRGB565(img)); err != nil {
		return err
	}

	return nil
}

func toRGB565(img image.Image) []byte {
	b := img.Bounds()
	rgb565 := make([]byte, b.Dx()*b.Dy()*2)

	i := 0

	for x := 0; x < b.Max.X; x++ {
		for y := 0; y < b.Max.Y; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			binary.BigEndian.PutUint16(rgb565[i:i+2], uint16((r<<8)&0b1111100000000000|(g<<3)&0b0000011111100000|(b>>3)&0b0000000000011111))
			i += 2
		}
	}

	return rgb565
}

func (d *Dev) send(command Command) error {
	if err := d.sendCommand([]byte{command.Command}); err != nil {
		return err
	}

	if command.Data != nil {
		if err := d.sendData(command.Data); err != nil {
			return err
		}
	}

	if command.Delay != time.Duration(0) {
		time.Sleep(command.Delay)
	}

	return nil
}

func (d *Dev) sendCommand(c []byte) error {
	if err := d.dc.Out(gpio.Low); err != nil {
		return err
	}
	return d.c.Tx(c, nil)
}

func (d *Dev) sendData(data []byte) error {
	if err := d.dc.Out(gpio.High); err != nil {
		return err
	}

	for len(data) != 0 {
		var chunk []byte

		if len(data) > d.maxTxSize {
			chunk, data = data[:d.maxTxSize], data[d.maxTxSize:]
		} else {
			chunk, data = data, nil
		}
		d.c.Tx(chunk, nil)
	}

	return nil
}

func (d *Dev) reset() error {
	if err := d.rst.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	if err := d.rst.Out(gpio.Low); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	if err := d.rst.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}
