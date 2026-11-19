package main

import (
	"fmt"
	"image/png"
	"os"
)

func main() {
	f, _ := os.Open(os.Args[1])
	im, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	b := im.Bounds()
	fmt.Println("size", b.Dx(), b.Dy())
	x := b.Dx() / 2
	prev := ""
	for y := b.Min.Y; y < b.Max.Y; y++ {
		r, g, bl, _ := im.At(x, y).RGBA()
		cur := fmt.Sprintf("(%d,%d,%d)", r>>8, g>>8, bl>>8)
		if cur != prev {
			fmt.Printf("y=%d %s\n", y, cur)
			prev = cur
		}
	}
}
