## goiv

Small and simple image viewer written in pure Go.


### Features

* Supports JPEG, PNG, GIF, BMP, PCX, TIFF, PBM, PGM, PPM, WEBP, PSD and TGA formats.
* Scales images to window size and preserves aspect ratio.
* Supports HTTP URLs passed as arguments.
* Cross-platform (note: on macOS you need to install [XQuartz](https://www.xquartz.org/)).


### Download

 - [Linux 64bit](https://github.com/gen2brain/goiv/releases/download/1.0/goiv-1.0-linux-64bit.tar.gz)
 - [Windows 32bit](https://github.com/gen2brain/goiv/releases/download/1.0/goiv-1.0-windows-32bit.zip)
 - [macOS 64bit](https://github.com/gen2brain/goiv/releases/download/1.0/goiv-1.0-darwin-64bit.zip)


### Installation

    go get -v github.com/gen2brain/goiv


This will install app in `$GOPATH/bin/goiv`.

Note: On Windows you need to generate manifest .syso file, use this instead:

    go get github.com/akavel/rsrc
    
    go get -d github.com/gen2brain/goiv
    go generate github.com/gen2brain/goiv
    go install github.com/gen2brain/goiv


### Keybindings

* j / Right / PageDown / Space

    `Next image`

* k / Left / PageUp

    `Previous image`

* f / F11

    `Fullscreen`

* [ / ]

    `Go 10 images back/forward`

* , / .

    `Go to first/last image`

* q / Escape

    `Quit`

* Enter

    `Print current image path to stdout`


### Example usage

* View all images in a directory

    `goiv /path/to/dir/*`

* View all JPEG's in all subdirectories

    `find . -iname "*.jpg" | goiv`

* Delete current image when enter is pressed

    `goiv * | xargs rm`

* Rotate current image when enter is pressed

    `goiv * | xargs -i convert -rotate 90 {} {}`


### Planned features

- [ ] draw in console on DRM/KMS and Framebuffer 
- [ ] flip image vertically/horizontally
- [ ] rotate image 90 degrees clockwise/counter-clockwise
