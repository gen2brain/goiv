// +build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"unsafe"

	"github.com/NeowayLabs/drm"
	"github.com/NeowayLabs/drm/mode"
	"github.com/edsrzf/mmap-go"
	"github.com/pkg/term"
)

type frameBuffer struct {
	id     uint32
	handle uint32
	data   []byte
	fb     *mode.FB
	size   uint64
	stride uint32
}

type msetData struct {
	mode      *mode.Modeset
	fb        frameBuffer
	savedCrtc *mode.Crtc
}

// displayDRM displays images on DRM.
func displayDRM(images []string) error {
	file, err := drm.OpenCard(0)
	if err != nil {
		return err
	}

	defer file.Close()

	if !drm.HasDumbBuffer(file) {
		return fmt.Errorf("drm device does not support dumb buffers")
	}

	modeset, err := mode.NewSimpleModeset(file)
	if err != nil {
		return fmt.Errorf("NewSimpleModeset: %s", err.Error())
	}

	var msets []msetData
	for _, mod := range modeset.Modesets {
		framebuf, err := createFramebuffer(file, &mod)
		if err != nil {
			cleanup(modeset, msets, file)
			return err
		}

		savedCrtc, err := mode.GetCrtc(file, mod.Crtc)
		if err != nil {
			cleanup(modeset, msets, file)
			return fmt.Errorf("GetCrtc: %s", err.Error())
		}

		err = mode.SetCrtc(file, mod.Crtc, framebuf.id, 0, 0, &mod.Conn, 1, &mod.Mode)
		if err != nil {
			cleanup(modeset, msets, file)
			return fmt.Errorf("SetCrtc: %s", err.Error())
		}

		msets = append(msets, msetData{
			mode:      &mod,
			fb:        framebuf,
			savedCrtc: savedCrtc,
		})
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

	update := func() error {
		var off uint32

		img, err := decode(images[idx])
		if err != nil {
			return err
		}

		img, err = scale(img, int(msets[0].fb.fb.Width), int(msets[0].fb.fb.Height))
		if err != nil {
			return err
		}

		bounds := img.Bounds()

		for j := 0; j < len(msets); j++ {
			mset := msets[j]
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					off = mset.fb.stride*uint32(y) + uint32(x)*4
					val := uint32((uint32(r) << 16) | (uint32(g) << 8) | uint32(b))
					*(*uint32)(unsafe.Pointer(&mset.fb.data[off])) = val
				}
			}
		}

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

	cleanup(modeset, msets, file)

	return nil
}

func createFramebuffer(file *os.File, dev *mode.Modeset) (frameBuffer, error) {
	fb, err := mode.CreateFB(file, dev.Width, dev.Height, 32)
	if err != nil {
		return frameBuffer{}, fmt.Errorf("CreateFB: %s", err.Error())
	}

	stride := fb.Pitch
	size := fb.Size
	handle := fb.Handle

	fbID, err := mode.AddFB(file, dev.Width, dev.Height, 24, 32, stride, handle)
	if err != nil {
		return frameBuffer{}, fmt.Errorf("AddFB: %s", err.Error())
	}

	offset, err := mode.MapDumb(file, handle)
	if err != nil {
		return frameBuffer{}, err
	}

	mm, err := mmap.MapRegion(file, int(size), mmap.RDWR, 0x01, int64(offset))
	if err != nil {
		return frameBuffer{}, fmt.Errorf("Map: %s", err.Error())
	}

	for i := uint64(0); i < size; i++ {
		mm[i] = 0
	}

	framebuf := frameBuffer{
		id:     fbID,
		handle: handle,
		data:   mm,
		fb:     fb,
		size:   size,
		stride: stride,
	}

	return framebuf, nil
}

func destroyFramebuffer(modeset *mode.SimpleModeset, mset msetData, file *os.File) error {
	handle := mset.fb.handle
	data := mset.fb.data
	fb := mset.fb

	err := mmap.MMap(data).Unlock()
	if err != nil {
		return fmt.Errorf("UnsafeUnmap: %s", err.Error())
	}

	err = mode.RmFB(file, fb.id)
	if err != nil {
		return fmt.Errorf("RmFB: %s", err.Error())
	}

	err = mode.DestroyDumb(file, handle)
	if err != nil {
		return fmt.Errorf("DestroyDumb: %s", err.Error())
	}

	return modeset.SetCrtc(mset.mode, mset.savedCrtc)
}

func cleanup(modeset *mode.SimpleModeset, msets []msetData, file *os.File) {
	for _, mset := range msets {
		err := destroyFramebuffer(modeset, mset, file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
	}
}
