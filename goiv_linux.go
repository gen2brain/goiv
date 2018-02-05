// +build linux

package main

import (
	"fmt"
	"os"
)

func display(images []string, width, height int) {
	if os.Getenv("DISPLAY") == "" {
		err := displayDRM(images)
		if err != nil {
			e := displayFB(images)
			if e != nil {
				fmt.Fprintf(os.Stderr, "%s; %s\n", err.Error(), e.Error())
			}
		}
	} else {
		displayX11(images, width, height)
	}
}
