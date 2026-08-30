// Package platesolve determines what direction a photo was taken in by
// matching bright points in the image against a catalog of known star
// positions (fieldwork/go/starcat), constrained to the hemisphere visible
// from the photo's EXIF location/time. This is a lightweight, from-scratch
// version of the geometric-hashing technique astrometry.net and Tetra3 use
// (quads of stars -> rotation/scale-invariant hash -> match -> verify),
// deliberately simplified because the search space here is already narrowed
// by known location/time rather than being a blind all-sky search.
package platesolve

import (
	"image"
	"math"
	"sort"
)

// Detection is one candidate star found in an image, in pixel coordinates
// (origin top-left, consistent with Go's image package).
type Detection struct {
	X, Y float64
	Flux float64 // sum of above-background brightness in the blob
}

// DetectStars finds bright, star-shaped blobs in img: pixels well above the
// image's overall background brightness, grouped into connected components,
// reduced to a flux-weighted centroid per component. It's tuned for sparse
// bright points on a dark background (a night-sky photo), not general-purpose
// blob detection — large or dim regions (clouds, the Moon, light pollution
// glow, streetlights) are filtered out by the size bounds below.
func DetectStars(img image.Image) []Detection {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}

	lum := make([]float64, w*h)
	var sum, sumSq float64
	for y := range h {
		for x := range w {
			l := luminance(img.At(b.Min.X+x, b.Min.Y+y))
			lum[y*w+x] = l
			sum += l
			sumSq += l * l
		}
	}
	n := float64(w * h)
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)

	// Stars are rare, isolated, bright outliers against a mostly-uniform
	// dark sky background, so a global threshold a few standard deviations
	// above the mean works well without needing a local/adaptive threshold.
	threshold := mean + 4*stddev
	if threshold <= mean {
		threshold = mean + 1
	}

	visited := make([]bool, w*h)
	minPixels := 1
	maxPixels := max((w*h)/200, 25) // reject anything covering >0.5% of the frame

	var detections []Detection

	// 4-connected flood fill over threshold-passing pixels.
	var stackX, stackY []int
	for y := range h {
		for x := range w {
			idx := y*w + x
			if visited[idx] || lum[idx] < threshold {
				continue
			}

			stackX, stackY = stackX[:0], stackY[:0]
			stackX, stackY = append(stackX, x), append(stackY, y)
			visited[idx] = true

			var pixCount int
			var fluxSum, wx, wy float64

			for len(stackX) > 0 {
				cx, cy := stackX[len(stackX)-1], stackY[len(stackY)-1]
				stackX, stackY = stackX[:len(stackX)-1], stackY[:len(stackY)-1]

				cIdx := cy*w + cx
				weight := lum[cIdx] - mean
				pixCount++
				fluxSum += weight
				wx += float64(cx) * weight
				wy += float64(cy) * weight

				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := cx+d[0], cy+d[1]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nIdx := ny*w + nx
					if visited[nIdx] || lum[nIdx] < threshold {
						continue
					}
					visited[nIdx] = true
					stackX, stackY = append(stackX, nx), append(stackY, ny)
				}
			}

			if pixCount < minPixels || pixCount > maxPixels || fluxSum <= 0 {
				continue
			}

			detections = append(detections, Detection{
				X:    wx / fluxSum,
				Y:    wy / fluxSum,
				Flux: fluxSum,
			})
		}
	}

	sort.Slice(detections, func(i, j int) bool { return detections[i].Flux > detections[j].Flux })
	return detections
}

func luminance(c interface{ RGBA() (r, g, b, a uint32) }) float64 {
	r, g, b, _ := c.RGBA()
	// Rec. 601 luma, on the 16-bit-per-channel values RGBA() returns.
	return 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
}
