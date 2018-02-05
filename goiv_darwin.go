// +build darwin

package main

func display(images []string, width, height int) {
	displayX11(images, width, height)
}
