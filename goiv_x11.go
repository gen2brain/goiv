package main

import (
	"fmt"
	"image"
	"image/draw"
	"io/ioutil"
	"os"

	"github.com/BurntSushi/xgb"
	mshm "github.com/BurntSushi/xgb/shm"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/gen2brain/shm"
)

const (
	none = 1 << iota
	loaded
	scaled
	drawn
)

// displayX11 displays images in X11 window.
func displayX11(images []string, width, height int) {
	xgb.Logger.SetOutput(ioutil.Discard)
	xgbutil.Logger.SetOutput(ioutil.Discard)

	X, err := xgbutil.NewConn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	useShm := true
	err = mshm.Init(X.Conn())
	if err != nil {
		useShm = false
	}

	keybind.Initialize(X)
	mousebind.Initialize(X)

	win, err := xwindow.Generate(X)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Generate: %s\n", err.Error())
		os.Exit(1)
	}

	defer win.Destroy()

	win.Create(X.RootWin(), 0, 0, width, height, xproto.CwBackPixel, 0x000000)
	win.Change(xproto.CwBackingStore, xproto.BackingStoreAlways)

	win.WMGracefulClose(func(w *xwindow.Window) {
		xevent.Detach(w.X, w.Id)
		keybind.Detach(w.X, w.Id)
		mousebind.Detach(w.X, w.Id)
		w.Destroy()
		xevent.Quit(w.X)
	})

	err = ewmh.WmWindowTypeSet(X, win.Id, []string{"_NET_WM_WINDOW_TYPE_DIALOG"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "WmWindowTypeSet: %s\n", err.Error())
	}

	win.Listen(xproto.EventMaskKeyPress, xproto.EventMaskButtonRelease, xproto.EventMaskStructureNotify, xproto.EventMaskExposure)

	idx := 0
	state := 0

	rect, err := win.Geometry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Geometry: %s\n", err.Error())
	}

	var shmId int
	var seg mshm.Seg
	var data []byte

	var img image.Image
	var ximg *xgraphics.Image

	newImage := func() *xgraphics.Image {
		var i *xgraphics.Image

		if useShm {
			shmSize := rect.Width() * rect.Height() * 4

			shmId, err = shm.Get(shm.IPC_PRIVATE, shmSize, shm.IPC_CREAT|0777)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Get: %s\n", err.Error())
			}

			seg, err = mshm.NewSegId(X.Conn())
			if err != nil {
				fmt.Fprintf(os.Stderr, "NewSegId: %s\n", err.Error())
			}

			data, err = shm.At(shmId, 0, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "At: %s\n", err.Error())
			}

			mshm.Attach(X.Conn(), seg, uint32(shmId), false)

			i = &xgraphics.Image{
				X:      X,
				Pixmap: 0,
				Pix:    data,
				Stride: 4 * rect.Width(),
				Rect:   image.Rect(0, 0, rect.Width(), rect.Height()),
				Subimg: false,
			}
		} else {
			i = xgraphics.New(X, image.Rect(0, 0, rect.Width(), rect.Height()))
		}

		return i
	}

	ximg = newImage()

	loadImage := func() {
		img, err = decode(images[idx], rect.Width(), rect.Height())
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			return
		}

		state |= loaded
	}

	scaleImage := func() {
		if ximg != nil {
			ximg.Destroy()
			ximg = nil
		}

		if useShm {
			mshm.Detach(X.Conn(), seg)
			shm.Rm(shmId)
			shm.Dt(data)
		}

		ximg = newImage()

		i, err := scale(img, rect.Width(), rect.Height())
		if err != nil {
			fmt.Fprintf(os.Stderr, "scale: %s\n", err.Error())
		}

		offset := (rect.Width() - i.Bounds().Max.X) / 2
		draw.Draw(ximg, i.Bounds().Add(image.Pt(offset, 0)), i, image.ZP, draw.Over)

		state |= scaled
	}

	drawImage := func() {
		title := fmt.Sprintf("%s [%d of %d] - %s (%dx%d)", appName, idx+1, len(images),
			images[idx], img.Bounds().Max.X, img.Bounds().Max.Y)

		err = ewmh.WmNameSet(ximg.X, win.Id, title)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WmNameSet: %s\n", err.Error())
		}

		if useShm {
			pid, err := xproto.NewPixmapId(ximg.X.Conn())
			if err != nil {
				fmt.Fprintf(os.Stderr, "NewPixmapId: %s\n", err.Error())
			}

			mshm.CreatePixmap(X.Conn(), pid, xproto.Drawable(ximg.X.RootWin()),
				uint16(ximg.Bounds().Dx()), uint16(ximg.Bounds().Dy()),
				ximg.X.Screen().RootDepth, seg, 0)

			ximg.Pixmap = pid

			mshm.PutImage(X.Conn(), xproto.Drawable(ximg.Pixmap), ximg.X.GC(),
				uint16(ximg.Bounds().Dx()), uint16(ximg.Bounds().Dy()),
				0, 0, 0, 0, 0, 0, ximg.X.Screen().RootDepth,
				xproto.ImageFormatZPixmap, 0, seg, 0)

			ximg.XExpPaint(win.Id, 0, 0)
		} else {
			err = ximg.CreatePixmap()
			if err != nil {
				fmt.Fprintf(os.Stderr, "CreatePixmap: %s\n", err.Error())
			}

			ximg.XDraw()
			ximg.XExpPaint(win.Id, 0, 0)
		}

		state |= drawn
	}

	update := func() {
		if state&loaded == 0 {
			loadImage()
			if img == nil {
				return
			}
		}
		if state&scaled == 0 {
			scaleImage()
		}
		if state&drawn == 0 {
			drawImage()
		}
	}

	cbKey := xevent.KeyPressFun(func(xu *xgbutil.XUtil, e xevent.KeyPressEvent) {
		if keybind.KeyMatch(X, "Escape", e.State, e.Detail) || keybind.KeyMatch(X, "q", e.State, e.Detail) {
			xevent.Quit(X)
		}

		if keybind.KeyMatch(xu, "Left", e.State, e.Detail) || keybind.KeyMatch(xu, "Page_Up", e.State, e.Detail) || keybind.KeyMatch(xu, "k", e.State, e.Detail) {
			if idx != 0 {
				idx -= 1
				state &= loaded
				state &= drawn
				update()
			}
		} else if keybind.KeyMatch(xu, "Right", e.State, e.Detail) || keybind.KeyMatch(xu, "Page_Down", e.State, e.Detail) ||
			keybind.KeyMatch(xu, "j", e.State, e.Detail) || keybind.KeyMatch(xu, " ", e.State, e.Detail) {
			if idx != len(images)-1 {
				idx += 1
				state &= loaded
				state &= drawn
				update()
			}
		}

		if keybind.KeyMatch(X, "F11", e.State, e.Detail) || keybind.KeyMatch(X, "f", e.State, e.Detail) || keybind.KeyMatch(X, "L1", e.State, e.Detail) {
			err = ewmh.WmStateReq(X, win.Id, ewmh.StateToggle, "_NET_WM_STATE_FULLSCREEN")
			if err != nil {
				fmt.Fprintf(os.Stderr, "WmStateReq: %s\n", err.Error())
			}
		}

		if keybind.KeyMatch(X, "[", e.State, e.Detail) {
			if idx-10 >= 0 {
				idx -= 10
				state &= loaded
				state &= drawn
				update()
			}
		} else if keybind.KeyMatch(X, "]", e.State, e.Detail) {
			if idx+10 <= len(images)-1 {
				idx += 10
				state &= loaded
				state &= drawn
				update()
			}
		}

		if keybind.KeyMatch(X, ",", e.State, e.Detail) {
			idx = 0
			state &= loaded
			state &= drawn
			update()
		} else if keybind.KeyMatch(X, ".", e.State, e.Detail) {
			idx = len(images) - 1
			state &= loaded
			state &= drawn
			update()
		}

		if keybind.KeyMatch(X, "Return", e.State, e.Detail) {
			fmt.Fprintf(os.Stdout, "%s\n", images[idx])
		}
	})

	cbBut := mousebind.ButtonReleaseFun(func(xu *xgbutil.XUtil, e xevent.ButtonReleaseEvent) {
		if e.Detail == 1 {
			if idx != len(images)-1 {
				idx += 1
				state &= loaded
				state &= drawn
				update()
			}
		} else if e.Detail == 3 {
			if idx != 0 {
				idx -= 1
				state &= loaded
				state &= drawn
				update()
			}
		}
	})

	cbCfg := xevent.ConfigureNotifyFun(func(xu *xgbutil.XUtil, e xevent.ConfigureNotifyEvent) {
		if rect.Width() != int(e.Width) || rect.Height() != int(e.Height) {
			state &= drawn
			state &= scaled

			rect, err = win.Geometry()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Geometry: %s\n", err.Error())
			}
		}
	})

	cbExp := xevent.ExposeFun(func(xu *xgbutil.XUtil, e xevent.ExposeEvent) {
		if e.ExposeEvent.Count == 0 {
			update()
		}
	})

	cbKey.Connect(X, win.Id)
	cbCfg.Connect(X, win.Id)
	cbBut.Connect(X, win.Id, "1", false, true)
	cbBut.Connect(X, win.Id, "3", false, true)
	cbExp.Connect(X, win.Id)

	win.Map()
	xevent.Main(X)

	if useShm {
		mshm.Detach(X.Conn(), seg)
		shm.Rm(shmId)
		shm.Dt(data)
	}
}
