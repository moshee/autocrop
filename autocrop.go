package autocrop

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sync"

	"ktkr.us/pkg/autocrop/util"
)

const (
	// ColorMax is maximum color value returned by the image/color API
	// functions, representing solid white.
	ColorMax = 0xFFFF
)

var (
	RED   = color.NRGBA{255, 0, 0, 255}
	GREEN = color.NRGBA{0, 255, 0, 255}
	BLUE  = color.NRGBA{0, 0, 255, 255}
)

// Transform is a transformation plan that, if used, should probably straighten
// the image it's associated with.
type Transform struct {
	Angle  float64         // rotate by this angle (in radians) to make it straight
	Bounds image.Rectangle // change the image bounds to this rectangle to fit
	// r^2 values of linear regression on each side; CSS box side order (T,R,B,L)
	Confidence [4]float64
}

// String returns the ImageMagick/GraphicsMagick flags required to perform the
// transformation.
//
// When ImageMagick rotates an image, it adds long thin triangles on each side
// to avoid losing any pixels in the original image. This adds more width that
// we need to add to the crop bounds.
func (t Transform) String() string {

	r := math.Sin(-t.Angle) / 2
	left := t.Bounds.Min.X + int(float64(t.Bounds.Dy())*r)
	top := t.Bounds.Min.Y + int(float64(t.Bounds.Dx())*r)

	return fmt.Sprintf("-rotate %f -crop %dx%d+%d+%d",
		util.Rad2deg(t.Angle), t.Bounds.Dx(), t.Bounds.Dy(), left, top)
}

// AnalyzeFile loads a PNG or JPEG file and performs Analyze on the resulting
// image.
func AnalyzeFile(filename string, thresh, fc float64, n int) (*Transform, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return Analyze(img, thresh, fc, n), nil
}

// Analyze examines a tilted image (book page scan) with a black border to
// determine its orientation and returns a transformation plan that will
// probably straighten and crop the black border off. It does not perform the
// transformation.
//
// It may fail. In particular, if the page itself has lots of black on the
// edges, the analysis will be confused. If the returned struct's Confidence
// values are less than 0.5 or so, manual intervention is advised.
//
// Theory of operation
//
// The analysis looks in from each edge of the image, searching for the edge of
// the page on each side. n samples are taken per side. More samples may mean
// more accuracy, but at the cost of CPU time and memory. Each sample gets its
// own goroutine.
//
// For each sample, the distance to the edge is discovered by tracking the
// derivative of the pixel values as a function of displacement and looking for
// anything above a certain threshold (thresh). If the change in values spikes
// above that, it indicates a page border.
//
// The samples from each side are arranged in order and a linear regression is
// calculated from them to determine how the edge of the page is angled. From
// this, an angle of rotation (from the slope) and the crop width (from the
// y-intercept) are determined.
//
// Assumptions
//
// The analysis assumes that the background is black and the page is mostly
// white around the edges. It only looks for rising edges (black to white).
// Falling edges will be ignored.
func Analyze(img image.Image, thresh, fc float64, n int) *Transform {
	var (
		a      = &analysis{img, thresh, fc}
		b      = a.img.Bounds()
		dx     = b.Dx()
		dy     = b.Dy()
		left   = make([]float64, n)
		right  = make([]float64, n)
		top    = make([]float64, n)
		bottom = make([]float64, n)
		wg     = new(sync.WaitGroup)
	)

	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			left[i], right[i] = a.analyzeX(i * dy / n)
			top[i], bottom[i] = a.analyzeY(i * dx / n)
			wg.Done()
		}(i)
	}

	wg.Wait()

	t := &Transform{}
	angles := make([]float64, 4)

	angles[0], t.Confidence[0], t.Bounds.Min.Y = analyzeResult(top, -1, n, dx, 0)
	angles[1], t.Confidence[1], t.Bounds.Max.X = analyzeResult(right, -1, n, dy, 1)
	angles[2], t.Confidence[2], t.Bounds.Max.Y = analyzeResult(bottom, 1, n, dx, 2)
	angles[3], t.Confidence[3], t.Bounds.Min.X = analyzeResult(left, 1, n, dy, 3)

	t.Bounds.Max.X = dx - t.Bounds.Max.X
	t.Bounds.Max.Y = dy - t.Bounds.Max.Y

	t.Angle = util.Mean(angles...)

	return t
}

// Interpret a sample set for the angle and crop size.
func analyzeResult(edges []float64, dir float64, n, d, i int) (angle, confidence float64, crop int) {
	q := 200
	lo, hi := util.Trim(edges, float64(q))

	edges = util.Lowpass(edges, .1)
	util.Clean(edges, float64(q), 24, 4, 8)
	a, b, r := util.LinearFit(edges)
	crop = int(a + b*float64(len(edges))/2)

	/*
		chart(edges, crop, lo, hi, func(x int) int {
			return int(b*float64(x) + a)
		}, fmt.Sprintf("side%d.png", i))
	*/

	edges = edges[lo:hi]

	angle = math.Atan(b * dir * float64(n) / float64(d))
	confidence = r

	return
}

type analysis struct {
	img    image.Image // image data
	thresh float64     // color value rising edge threshold
	fc     float64     // cutoff frequency for low-pass denoise filter
}

// grayAt returns the image's gray value at the x, y coordinate.
// This function is a pain point due to I2T conversions and sheer # of calls.
func (a *analysis) grayAt(x, y int) uint8 {
	if p, ok := a.img.(*image.Gray); ok {
		return p.Pix[p.PixOffset(x, y)]
	}

	r, g, b, _ := a.img.At(x, y).RGBA()
	return uint8((r + g + b) / 3) // dumb blend, no need for visual aesthetics
}

func (a *analysis) analyzeX(y int) (left, right float64) {
	dx := a.img.Bounds().Dx()
	m := dx / 16 // this is the portion of the image that is processed.
	samples := make([]float64, m)

	a.sampleX(samples, y, 0, m, 1)
	left = a.search(samples)

	a.sampleX(samples, y, dx, dx-m, -1)
	right = a.search(samples)

	return
}

func (a *analysis) analyzeY(x int) (top, bottom float64) {
	dy := a.img.Bounds().Dy()
	m := dy / 16
	samples := make([]float64, m)

	a.sampleY(samples, x, 0, m, 1)
	top = a.search(samples)

	a.sampleY(samples, x, dy, dy-m, -1)
	bottom = a.search(samples)

	return
}

func (a *analysis) sampleX(samples []float64, y, start, end, delta int) {
	for x, i := start, 0; x != end; x, i = x+delta, i+1 {
		samples[i] = float64(a.grayAt(x, y))
	}
}

func (a *analysis) sampleY(samples []float64, x, start, end, delta int) {
	for y, i := start, 0; y != end; y, i = y+delta, i+1 {
		samples[i] = float64(a.grayAt(x, y))
	}
}

// search a contiguous set of samples for a rising edge.
func (a *analysis) search(samples []float64) (edge float64) {
	samples = util.Lowpass(samples, a.fc)
	d := util.Differentiate(samples)

	// find the center of the peak in the derivative which indicates where a
	// page edge is
findPeak:
	for i, sample := range d {
		if sample > a.thresh {
			max := sample
			maxI := i

		findPeakFallingEdge:
			for ; i < len(d); i++ {
				sample = d[i]
				if sample <= a.thresh {
					break findPeakFallingEdge
				}
				if sample > max {
					max = sample
					maxI = i
				}
			}

			edge = float64(maxI)
			break findPeak
		}
	}

	return
}

func chart(samples []float64, cutoff, lo, hi int, line func(int) int, name string) {
	img := image.NewNRGBA(image.Rect(0, 0, len(samples), 200))
	util.Histo(img, samples, color.NRGBA{180, 180, 255, 255}, color.White, color.White, nil)
	util.RectOver(img, lo, hi, color.NRGBA{0, 255, 0, 30})
	util.Line(img, line, color.Black)
	util.DashedLine(img, 200-cutoff, RED)
	util.WriteImage(img, name)
}
