// Package util contains some utility functions for package autocrop that might
// be useful for other things. Some of them make assumptions specific to
// autocrop, though.
package util

// util.go contains functions related to analyzing and cleaning noise from
// sample sets.

import "math"

// Scale normalizes a set of values so that its highest and lowest values
// correspond to hi and lo.
func Scale(xs []float64, lo, hi float64) {
	min, max := MinMax(xs)
	a := (hi - lo) / (max - min)
	dy := (lo - min) * a
	for i := range xs {
		xs[i] = xs[i]*a + dy
	}
}

// Lowpass applies a discrete low-pass filter with cutoff frequency fc to x.
func Lowpass(x []float64, fc float64) (y []float64) {
	y = make([]float64, len(x))
	RC := 1.0 / (2 * math.Pi * fc)
	α := 1.0 / (RC + 1.0)
	y[0] = x[0]
	for t := 1; t < len(x); t++ {
		y[t] = y[t-1] + α*(x[t]-y[t-1])
	}
	return y
}

// Differentiate performs a discrete signal differentiation over xs by taking
// the slope between the two immediately adjacent samples for every sample.
func Differentiate(xs []float64) []float64 {
	if len(xs) == 0 {
		return nil
	}

	ddx := make([]float64, len(xs))

	ddx[0] = xs[1] - xs[0]
	for i := 1; i < len(ddx)-1; i++ {
		ddx[i] = (xs[i+1] - xs[i-1]) / 2
	}
	ddx[len(ddx)-1] = xs[len(xs)-1] - xs[len(xs)-2]

	return Lowpass(ddx, 1./10.)
}

// Mean finds the mean of a set of values.
func Mean(xs ...float64) (a float64) {
	for _, x := range xs {
		a += x
	}
	a /= float64(len(xs))
	return
}

// MinMax finds the min and max of a set of values.
func MinMax(xs []float64) (min, max float64) {
	for _, x := range xs {
		if x > max {
			max = x
		} else if x < min {
			min = x
		}
	}

	return
}

// Rad2deg converts from radians to degrees.
func Rad2deg(rad float64) float64 {
	return rad * 180 / math.Pi
}

// LinearFit returns the slope of a naïve linear regression on xs. It ignores
// values equal to zero.
func LinearFit(xs []float64) (alpha, beta, r2 float64) {
	var (
		xy, sx, sy, x2, y2 float64
		n                  = float64(len(xs))
	)
	for i, y := range xs {
		if y == 0 {
			n -= 1
			continue
		}
		x := float64(i)
		xy += x * y
		sx += x
		sy += y
		x2 += x * x
		y2 += y * y
	}
	xy /= n
	sx /= n
	sy /= n
	x2 /= n
	y2 /= n

	beta = (xy - sx*sy) / (x2 - sx*sx)
	alpha = sy - beta*sx
	r := (xy - sx*sy) / math.Sqrt((x2-sx*sx)*(y2-sy*sy))
	r2 = r * r
	return
}

// Clean tries to recover a clean signal with a straight slope from a garbled
// one. It employs several methods to attempt to detect irregular values and
// allow the "correct" signal to dominate.
func Clean(xs []float64, cutoff, regressionDev, chunkMeanDev float64, chunkSize int) {
	// Split up the signal into chunks and calculate the average absolute
	// deviation across each. Chunks with a relatively high value are zeroed
	// out.
	var chunk []float64
	zeroes := make([]float64, chunkSize)
	for t := 0; t < len(xs); t += chunkSize {
		if len(xs)-t < 8 {
			chunk = xs[t:]
		} else {
			chunk = xs[t : t+chunkSize]
		}

		dev := AvgAbsDev(chunk)
		if dev > chunkMeanDev {
			copy(chunk, zeroes)
		}
	}

	// calculate a linear regression and find the samples that are too far away
	// from it. Then zero them out.
	a, b, _ := LinearFit(xs)
	for t, y := range xs {
		expected := a + b*float64(t)
		if math.Abs(expected-y) > regressionDev {
			xs[t] = 0
		}
	}

	// The linear fit ignores zero samples. So it'll only recalculate from the
	// "valid" samples. Hopefully. After that we put all the previously zeroed
	// out values back in, aligned perfectly with the new linear fit.
	a, b, _ = LinearFit(xs)
	for t, y := range xs {
		if y == 0 {
			xs[t] = a + b*float64(t)
		}
	}
}

// AvgAbsDev calculates the average absolute deviation from the mean within a
// sample.
func AvgAbsDev(xs []float64) float64 {
	mean := Mean(xs...)
	dev := 0.

	for _, y := range xs {
		dev += math.Abs(y - mean)
	}

	return dev / float64(len(xs))
}

// Trim removes samples from either side of a signal that exceed thresh or are
// zero.
func Trim(xs []float64, thresh float64) (lo, hi int) {
	hi = len(xs)

	for t, y := range xs {
		if y < thresh && y > 0 {
			//if y > 0 {
			lo = t
			break
		}
	}
	for t := len(xs); t > 0; t-- {
		y := xs[t-1]
		if y < thresh && y > 0 {
			//if y > 0 {
			hi = t
			break
		}
	}

	return
}
