// +build windows

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
)

//go:generate rsrc -manifest manifest/goiv.exe.manifest -o goiv_windows.syso

func display(images []string, width, height int) {
	mw := new(Window)
	mw.images = images

	keyEvent := func(key walk.Key) {
		switch key {
		case walk.KeyQ, walk.KeyEscape:
			mw.Close()
		case walk.KeyPrior, walk.KeyLeft, walk.KeyK, walk.KeyLButton:
			if mw.idx != 0 {
				mw.idx -= 1
				mw.drawImageError()
			}
		case walk.KeyNext, walk.KeyRight, walk.KeyJ, walk.KeyRButton, walk.KeySpace:
			if mw.idx != len(mw.images)-1 {
				mw.idx += 1
				mw.drawImageError()
			}
		case walk.KeyF11, walk.KeyF:
			mw.SetFullscreen(!mw.Fullscreen())
		case walk.KeyOEM4:
			if mw.idx-10 >= 0 {
				mw.idx -= 10
				mw.drawImageError()
			}
		case walk.KeyOEM6:
			if mw.idx+10 <= len(mw.images)-1 {
				mw.idx += 10
				mw.drawImageError()
			}
		case walk.KeyOEMComma:
			mw.idx = 0
			mw.drawImageError()
		case walk.KeyOEMPeriod:
			mw.idx = len(mw.images) - 1
			mw.drawImageError()
		case walk.KeyReturn:
			fmt.Fprintf(os.Stdout, "%s\n", mw.images[mw.idx])
		}
	}

	mouseEvent := func(x, y int, button walk.MouseButton) {
		switch button {
		case walk.RightButton:
			if mw.idx != 0 {
				mw.idx -= 1
				mw.drawImageError()
			}
		case walk.LeftButton:
			if mw.idx != len(mw.images)-1 {
				mw.idx += 1
				mw.drawImageError()
			}
		}
	}

	if err := (decl.MainWindow{
		AssignTo:    &mw.MainWindow,
		OnKeyDown:   keyEvent,
		OnMouseDown: mouseEvent,
		MinSize:     decl.Size{320, 240},
		Size:        decl.Size{width, height},
		Layout:      decl.VBox{MarginsZero: true, SpacingZero: true},
		Children: []decl.Widget{
			decl.ImageView{
				AssignTo:    &mw.imageView,
				Background:  decl.SolidColorBrush{Color: walk.RGB(0, 0, 0)},
				OnKeyDown:   keyEvent,
				OnMouseDown: mouseEvent,
			},
		},
	}.Create()); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	mw.drawImageError()

	mw.Run()
}

type Window struct {
	*walk.MainWindow

	image     walk.Image
	imageView *walk.ImageView

	idx    int
	images []string
}

func (mw *Window) drawImageError() {
	if err := mw.drawImage(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	}
}

func (mw *Window) drawImage() error {
	var err error

	if mw.image != nil {
		mw.image.Dispose()
		mw.image = nil
	}

	if isURL(mw.images[mw.idx]) {
		b, err := downloadURL(mw.images[mw.idx])
		if err != nil {
			return err
		}

		tmp, err := ioutil.TempFile("", "goiv")
		if err != nil {
			return err
		}

		_, err = tmp.Write(b)
		if err != nil {
			return err
		}

		err = tmp.Close()
		if err != nil {
			return err
		}

		defer os.Remove(tmp.Name())

		mw.image, err = walk.NewImageFromFile(tmp.Name())
		if err != nil {
			return err
		}
	} else {
		mw.image, err = walk.NewImageFromFile(mw.images[mw.idx])
		if err != nil {
			return err
		}
	}

	var succeeded bool
	defer func() {
		if !succeeded {
			if mw.image != nil {
				mw.image.Dispose()
			}
		}
	}()

	if mw.imageView == nil {
		mw.imageView, err = walk.NewImageView(mw)
		if err != nil {
			return err
		}
	}

	mw.imageView.SetMode(walk.ImageViewModeShrink)
	if err = mw.imageView.SetImage(mw.image); err != nil {
		return err
	}

	title := fmt.Sprintf("%s [%d of %d] - %s (%dx%d)", appName, mw.idx+1, len(mw.images),
		mw.images[mw.idx], mw.image.Size().Width, mw.image.Size().Height)
	mw.SetTitle(title)

	succeeded = true

	return nil
}
