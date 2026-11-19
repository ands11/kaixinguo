package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	im, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	b := im.Bounds()
	fmt.Println("size", b.Dx(), b.Dy())
	xs := []int{b.Min.X + 2, b.Dx() / 2, b.Max.X - 3}
	for _, x := range xs {
		fmt.Print("col x=", x, ": ")
		for y := b.Min.Y; y < b.Min.Y+12 && y < b.Max.Y; y++ {
			r, g, bl, _ := im.At(x, y).RGBA()
			fmt.Printf("%d:(%d,%d,%d) ", y, r>>8, g>>8, bl>>8)
		}
		fmt.Println()
	}
	_ = image.Rect
}
