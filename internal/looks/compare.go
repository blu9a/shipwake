package looks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // looks may be GIF, JPEG or PNG
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
)

// Threshold is how different two pixels' colors must be to count as changed: 0.1 of the largest
// perceptual difference (YIQ color space, the colors laid over white by their transparency). It is fixed,
// so the same two screenshots always get the same verdict.
const Threshold = 0.1

// maxDelta is the largest YIQ difference, between black and white.
const maxDelta = 35215.0

// Highlight is the color of the outline around changed pixels in a diff image; inside it they are Tint,
// so the highlight shows where the look changed without hiding what it changed to.
var (
	Highlight = color.NRGBA{R: 0xE5, G: 0x48, B: 0x4D, A: 0xFF}
	Tint      = color.NRGBA{R: 0xE5, G: 0x48, B: 0x4D, A: 0x40}
)

// Image is a decoded screenshot and the hash of its file.
type Image struct {
	Hash string
	Img  *image.NRGBA
}

// Load reads and decodes a screenshot file.
func Load(path string) (Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Image{}, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Image{}, fmt.Errorf("%s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return Image{Hash: hex.EncodeToString(sum[:]), Img: toNRGBA(img)}, nil
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Rect, img, b.Min, draw.Src)
	return n
}

// Result is what Compare found.
type Result struct {
	Pixels int          // pixels that differ, not counting anti-aliasing
	Diff   *image.NRGBA // the differing pixels on a transparent image, as large as the larger image: outlined in Highlight, filled with Tint
}

// Compare compares a screenshot with the approved one, pixel by pixel.
//
// A pixel differs when its color is further from the approved pixel's than Threshold, unless it is
// anti-aliasing: an edge pixel shaded between its darkest and brightest neighbours, where one of those
// neighbours is part of a flat area in both images (Vyšniauskas, "Anti-aliased pixel and intensity slope
// detector", 2009). Font smoothing that moves by a sub-pixel is tolerated this way; a changed color, size
// or position is not. Where the images' sizes differ, every pixel outside the area both cover differs.
func Compare(now, approved *image.NRGBA) Result {
	w := max(now.Rect.Dx(), approved.Rect.Dx())
	h := max(now.Rect.Dy(), approved.Rect.Dy())
	cw := min(now.Rect.Dx(), approved.Rect.Dx())
	ch := min(now.Rect.Dy(), approved.Rect.Dy())
	res := Result{Diff: image.NewNRGBA(image.Rect(0, 0, w, h))}
	limit := maxDelta * Threshold * Threshold
	changed := make([]bool, w*h)
	for y := range h {
		for x := range w {
			if x >= cw || y >= ch {
				res.Pixels++
				changed[y*w+x] = true
				continue
			}
			a, b := now.PixOffset(x, y), approved.PixOffset(x, y)
			if bytes.Equal(now.Pix[a:a+4], approved.Pix[b:b+4]) {
				continue
			}
			if math.Abs(delta(now, approved, a, b, false)) <= limit {
				continue
			}
			if antialiased(now, x, y, cw, ch, approved) || antialiased(approved, x, y, cw, ch, now) {
				continue
			}
			res.Pixels++
			changed[y*w+x] = true
		}
	}
	// A changed pixel up to two pixels from an unchanged one, or from the image's edge, outlines the
	// change: two pixels, so the outline stays visible when the screenshot is shown smaller.
	at := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && changed[y*w+x] }
	for y := range h {
		for x := range w {
			if !changed[y*w+x] {
				continue
			}
			c := Tint
			if !at(x-1, y) || !at(x+1, y) || !at(x, y-1) || !at(x, y+1) || !at(x-2, y) || !at(x+2, y) || !at(x, y-2) || !at(x, y+2) {
				c = Highlight
			}
			res.Diff.SetNRGBA(x, y, c)
		}
	}
	return res
}

// EncodePNG writes an image as PNG.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img)
	return buf.Bytes(), err
}

// antialiased reports whether the pixel at x, y of img is likely anti-aliasing: it has at most two
// identical neighbours, both darker and brighter ones, and its darkest or brightest neighbour sits in a
// flat area in both images.
func antialiased(img *image.NRGBA, x1, y1, w, h int, other *image.NRGBA) bool {
	x0, y0 := max(x1-1, 0), max(y1-1, 0)
	x2, y2 := min(x1+1, w-1), min(y1+1, h-1)
	pos := img.PixOffset(x1, y1)
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}
	var lo, hi float64
	var loX, loY, hiX, hiY int
	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			d := delta(img, img, pos, img.PixOffset(x, y), true)
			switch {
			case d == 0:
				zeroes++
				if zeroes > 2 {
					return false // several identical neighbours: part of a flat area, not an edge
				}
			case d < lo:
				lo, loX, loY = d, x, y
			case d > hi:
				hi, hiX, hiY = d, x, y
			}
		}
	}
	if lo == 0 || hi == 0 {
		return false // no darker or no brighter neighbour: not between two shades
	}
	return flat(img, loX, loY, w, h) && flat(other, loX, loY, w, h) ||
		flat(img, hiX, hiY, w, h) && flat(other, hiX, hiY, w, h)
}

// flat reports whether a pixel has more than two identical neighbours.
func flat(img *image.NRGBA, x1, y1, w, h int) bool {
	x0, y0 := max(x1-1, 0), max(y1-1, 0)
	x2, y2 := min(x1+1, w-1), min(y1+1, h-1)
	pos := img.PixOffset(x1, y1)
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}
	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			if bytes.Equal(img.Pix[pos:pos+4], img.Pix[img.PixOffset(x, y):img.PixOffset(x, y)+4]) {
				zeroes++
			}
			if zeroes > 2 {
				return true
			}
		}
	}
	return false
}

// delta is the perceptual difference between two pixels in YIQ, each laid over white by its
// transparency; negative when the first is brighter. yOnly gives the brightness difference alone.
// Every product is converted explicitly, so no platform fuses them and the result is the same everywhere.
func delta(a, b *image.NRGBA, i, j int, yOnly bool) float64 {
	r1, g1, b1 := over(a.Pix[i : i+4])
	r2, g2, b2 := over(b.Pix[j : j+4])
	y1, y2 := yiqY(r1, g1, b1), yiqY(r2, g2, b2)
	y := y1 - y2
	if yOnly {
		return y
	}
	in := yiqI(r1, g1, b1) - yiqI(r2, g2, b2)
	q := yiqQ(r1, g1, b1) - yiqQ(r2, g2, b2)
	d := float64(0.5053*y*y) + float64(0.299*in*in) + float64(0.1957*q*q)
	if y1 > y2 {
		return -d
	}
	return d
}

func over(p []byte) (r, g, b float64) {
	r, g, b = float64(p[0]), float64(p[1]), float64(p[2])
	if p[3] < 255 {
		a := float64(p[3]) / 255
		r = 255 + float64((r-255)*a)
		g = 255 + float64((g-255)*a)
		b = 255 + float64((b-255)*a)
	}
	return r, g, b
}

func yiqY(r, g, b float64) float64 {
	return float64(r*0.29889531) + float64(g*0.58662247) + float64(b*0.11448223)
}

func yiqI(r, g, b float64) float64 {
	return float64(r*0.59597799) - float64(g*0.27417610) - float64(b*0.32180189)
}

func yiqQ(r, g, b float64) float64 {
	return float64(r*0.21147017) - float64(g*0.52261711) + float64(b*0.31114694)
}
