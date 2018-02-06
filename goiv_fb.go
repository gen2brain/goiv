// +build linux

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"os"

	"github.com/jteeuwen/framebuffer"
	"github.com/pkg/term"
)

// displayFB displays images on Linux framebuffer.
func displayFB(images []string) error {
	canvas, err := framebuffer.Open(nil)
	if err != nil {
		return fmt.Errorf("Open: %s", err.Error())
	}

	defer canvas.Close()

	mode, err := canvas.CurrentMode()
	if err != nil {
		return fmt.Errorf("CurrentMode: %s", err.Error())
	}

	fb, err := canvas.Image()
	if err != nil {
		return fmt.Errorf("Image: %s", err.Error())
	}

	t, err := term.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("Open: %s", err.Error())
	}

	err = t.SetRaw()
	if err != nil {
		return fmt.Errorf("SetRaw: %s", err.Error())
	}

	defer t.Restore()
	defer t.Close()

	idx := 0
	var img image.Image

	update := func() error {
		img, err = decode(images[idx])
		if err != nil {
			return err
		}

		img, err = scale(img, mode.Geometry.XRes, mode.Geometry.YRes)
		if err != nil {
			return err
		}

		fbb := fb.Bounds()
		imgb := img.Bounds()

		imgb = imgb.Add(image.Point{
			(fbb.Dx() / 2) - (imgb.Dx() / 2),
			(fbb.Dy() / 2) - (imgb.Dy() / 2),
		})

		draw.Draw(fb, imgb, img, image.ZP, draw.Src)

		return nil
	}

	char := func() ([]byte, error) {
		bytes := make([]byte, 3)
		n, err := t.Read(bytes)
		if err != nil {
			return nil, err
		}

		return bytes[0:n], nil
	}

	wait := func() {
		for {
			c, err := char()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			}

			switch {
			case bytes.Equal(c, []byte{3}), bytes.Equal(c, []byte{113}), bytes.Equal(c, []byte{27}): // ctrl+c, q, Esc
				return
			case bytes.Equal(c, []byte{27, 91, 68}), bytes.Equal(c, []byte{27, 91, 53}), bytes.Equal(c, []byte{107}): // Left, Page_Up, k
				if idx != 0 {
					idx -= 1
					update()
				}
			case bytes.Equal(c, []byte{27, 91, 67}), bytes.Equal(c, []byte{27, 91, 54}), bytes.Equal(c, []byte{106}), bytes.Equal(c, []byte{32}): // Right, Page_Down, j, Space
				if idx != len(images)-1 {
					idx += 1
					update()
				}
			case bytes.Equal(c, []byte{91}): // [
				if idx+10 <= len(images)-1 {
					idx += 10
					update()
				}
			case bytes.Equal(c, []byte{93}): // ]
				if idx+10 <= len(images)-1 {
					idx += 10
					update()
				}
			case bytes.Equal(c, []byte{44}): // ,
				idx = 0
				update()
			case bytes.Equal(c, []byte{46}): // .
				idx = len(images) - 1
				update()
			case bytes.Equal(c, []byte{13}): // Return
				fmt.Fprintf(os.Stdout, "%s\n", images[idx])
			}
		}
	}

	err = update()
	if err != nil {
		return err
	}

	wait()

	return nil
}
