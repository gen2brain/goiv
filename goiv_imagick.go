// +build imagick

package main

import (
	"fmt"
	"image"
	"io/ioutil"
	"math"
	"net/http"

	"gopkg.in/gographics/imagick.v3/imagick"
)

func init() {
	imagick.Initialize()
}

// decode decodes image.
func decode(filename string) (image.Image, error) {
	if isURL(filename) {
		return decodeURL(filename)
	}

	return decodeFile(filename)
}

// decodeFile decodes image file.
func decodeFile(filename string) (image.Image, error) {
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	err := mw.ReadImage(filename)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", filename, err)
	}

	w := mw.GetImageWidth()
	h := mw.GetImageHeight()

	i := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	pix, err := mw.ExportImagePixels(0, 0, w, h, "RGBA", imagick.PIXEL_CHAR)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", filename, err)
	}

	i.Pix = pix.([]byte)

	return i, nil
}

// decodeURL decodes image from URL.
func decodeURL(url string) (image.Image, error) {
	b, err := downloadURL(url)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", url, err)
	}

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	err = mw.ReadImageBlob(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", url, err)
	}

	w := mw.GetImageWidth()
	h := mw.GetImageHeight()

	i := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	pix, err := mw.ExportImagePixels(0, 0, w, h, "RGBA", imagick.PIXEL_CHAR)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", url, err)
	}

	i.Pix = pix.([]byte)

	return i, nil
}

// downloadURL returns bytes from URL.
func downloadURL(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", url, err)
	}

	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}

// scale scales image keeping aspect ratio.
func scale(img image.Image, width, height int) (image.Image, error) {
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	bg := imagick.NewPixelWand()
	bg.SetColor("black")
	defer bg.Destroy()

	err := mw.NewImage(uint(w), uint(h), bg)
	if err != nil {
		return nil, fmt.Errorf("NewImage: %s", err.Error())
	}

	in := img.(*image.RGBA)

	err = mw.ImportImagePixels(0, 0, uint(w), uint(h), "RGBA", imagick.PIXEL_CHAR, in.Pix)
	if err != nil {
		return nil, fmt.Errorf("ImportImagePixels: %s", err.Error())
	}

	aw, ah := aspect(w, h, width, height)

	err = mw.ResizeImage(uint(aw), uint(ah), imagick.FILTER_BOX)
	if err != nil {
		return nil, fmt.Errorf("ResizeImage: %s", err.Error())
	}

	out := image.NewRGBA(image.Rect(0, 0, int(aw), int(ah)))
	pix, err := mw.ExportImagePixels(0, 0, aw, ah, "RGBA", imagick.PIXEL_CHAR)
	if err != nil {
		return nil, fmt.Errorf("ExportImagePixels: %s", err.Error())
	}

	out.Pix = pix.([]byte)

	return out, nil
}

// aspect preserves image aspect ratio.
func aspect(iw, ih, w, h int) (uint, uint) {
	mf := math.Min(float64(w)/float64(iw), float64(h)/float64(ih))
	width := float64(iw) * mf
	height := float64(ih) * mf
	return uint(width), uint(height)
}
