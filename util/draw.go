package util

// draw.go contains types and routines for drawing histograms on the image/draw
// API.

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
)

type shape struct {
	c color.Color
}

func (shape) ColorModel() color.Model {
	return color.NRGBAModel
}

// RectOver draws a c-colored rectangle over img with the tops and bottoms
// flush and vertical sides corresponding to x coordinates lo and hi.
func RectOver(img draw.Image, lo, hi int, c color.Color) {
	overlayRect := image.Rect(lo, 0, hi, img.Bounds().Dy())
	src := image.NewUniform(c)
	draw.Draw(img, overlayRect, src, image.ZP, draw.Over)
}

type dashedLine struct {
	shape
	pattern []bool
	y       int
}

func (d dashedLine) Bounds() image.Rectangle {
	return image.Rect(-1e9, d.y, 1e9, d.y+1)
}

func (d dashedLine) At(x, y int) color.Color {
	if d.pattern[x%len(d.pattern)] {
		return d.c
	}
	return color.Transparent
}

// DashedLine draws a c-colored dashed horizontal line across img at a given y
// coordinate according to a pattern of bools for each pixel. The default
// pattern is
//
//     "-----     "
//
// That is, five pixel dash and five pixel gap.
func DashedLine(img draw.Image, y int, c color.Color, pattern ...bool) {
	if pattern == nil {
		pattern = []bool{true, true, true, true, true, false, false, false, false, false}
	}

	d := dashedLine{shape{c}, pattern, y}
	draw.Draw(img, img.Bounds(), d, image.ZP, draw.Over)
}

type dashedColumn struct {
	shape
	pattern []bool
	x       int
}

func (c dashedColumn) Bounds() image.Rectangle {
	return image.Rect(c.x, -1e9, c.x+1, 1e9)
}

func (c dashedColumn) At(x, y int) color.Color {
	if c.pattern[y%len(c.pattern)] {
		return c.c
	}
	return color.Transparent
}

func DashedColumn(img draw.Image, x int, c color.Color, pattern ...bool) {
	if pattern == nil {
		pattern = []bool{true, true, true, true, true, false, false, false, false, false}
	}

	col := dashedColumn{shape{c}, pattern, x}
	draw.Draw(img, img.Bounds(), col, image.ZP, draw.Over)
}

type line struct {
	shape
	f      func(int) int
	bottom int
}

func (l line) Bounds() image.Rectangle {
	return image.Rect(-1e9, -1e9, 1e9, 1e9)
}

func (l line) At(x, y int) color.Color {
	if l.f(x) == l.bottom-y {
		return l.c
	}

	return color.Transparent
}

func Line(img draw.Image, f func(int) int, c color.Color) {
	l := line{shape{c}, f, img.Bounds().Max.Y}
	draw.Draw(img, img.Bounds(), l, image.ZP, draw.Over)
}

type histogram struct {
	shape
	samples  []float64
	pos, neg color.Color
	bounds   image.Rectangle
	// should return a value clamped between 0 and the image height
	transformer func(float64) float64
}

func (h *histogram) Bounds() image.Rectangle {
	return h.bounds
}

func (h *histogram) At(x, y int) color.Color {
	val := h.samples[x]
	c := h.pos

	if h.transformer != nil {
		val = h.transformer(val)
	}
	if val < 0 {
		c = h.neg
		val = -val
	}

	if y > h.bounds.Dy()-int(val) {
		return c
	}
	return h.c
}

// Histo draws a histogram over img with 1-pixel wide bars corresponding to
// values in samples. Positive values are pos-colored, negative values are
// neg-colored, and the rest of the pixels are back-colored. If transformer is
// not nil, it is applied to each value of samples before using.
func Histo(img draw.Image, samples []float64, pos, neg, back color.Color, transformer func(float64) float64) {
	h := &histogram{shape{back}, samples, pos, neg, img.Bounds(), transformer}
	draw.Draw(img, img.Bounds(), h, image.ZP, draw.Over)
}

// WriteImage writes an image to a png file.
func WriteImage(img image.Image, filename string) {
	out, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}

	if err = png.Encode(out, img); err != nil {
		log.Fatal(err)
	}
}
