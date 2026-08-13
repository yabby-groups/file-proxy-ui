//go:build tools

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type canvas struct {
	img   *image.RGBA
	scale float64
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if err := generate(root); err != nil {
		panic(err)
	}
}

func generate(root string) error {
	buildDir := filepath.Join(root, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(buildDir, "windows"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(buildDir, "linux"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "frontend", "src", "assets"), 0o755); err != nil {
		return err
	}

	appIcon := renderIcon(1024)
	if err := writePNG(filepath.Join(buildDir, "appicon.png"), appIcon); err != nil {
		return err
	}
	if err := writePNG(filepath.Join(root, "frontend", "src", "assets", "logo.png"), renderIcon(512)); err != nil {
		return err
	}

	linuxSizes := []int{16, 24, 32, 48, 64, 128, 256, 512}
	for _, size := range linuxSizes {
		name := filepath.Join(buildDir, "linux", "appicon-"+itoa(size)+".png")
		if err := writePNG(name, renderIcon(size)); err != nil {
			return err
		}
	}

	icoSizes := []int{16, 24, 32, 48, 64, 128, 256}
	if err := writeICO(filepath.Join(buildDir, "windows", "appicon.ico"), icoSizes); err != nil {
		return err
	}
	return writeICNS(filepath.Join(buildDir, "darwin", "appicon.icns"))
}

func renderIcon(size int) *image.RGBA {
	base := renderMasterIcon()
	if size == 1024 {
		return base
	}
	return resizeImage(base, size)
}

func renderMasterIcon() *image.RGBA {
	size := 1024
	super := 4
	c := &canvas{
		img:   image.NewRGBA(image.Rect(0, 0, size*super, size*super)),
		scale: float64(super),
	}
	c.clear(color.NRGBA{R: 0, G: 0, B: 0, A: 0})

	bgTop := color.NRGBA{R: 13, G: 113, B: 95, A: 255}
	bgBottom := color.NRGBA{R: 23, G: 32, B: 51, A: 255}
	c.roundedRect(64, 64, 896, 896, 210, bgBottom)
	c.roundedRect(64, 64, 896, 710, 210, bgTop)
	c.circle(820, 190, 74, color.NRGBA{R: 71, G: 209, B: 159, A: 58})
	c.circle(222, 808, 118, color.NRGBA{R: 0, G: 0, B: 0, A: 35})

	shadow := color.NRGBA{R: 0, G: 0, B: 0, A: 42}
	c.roundedRect(306, 214, 734, 770, 58, shadow)
	file := color.NRGBA{R: 241, G: 248, B: 246, A: 255}
	c.roundedRect(286, 190, 714, 746, 58, file)
	c.poly([]point{{584, 190}, {714, 320}, {584, 320}}, color.NRGBA{R: 202, G: 231, B: 223, A: 255})
	c.poly([]point{{584, 190}, {714, 320}, {626, 300}, {604, 212}}, color.NRGBA{R: 255, G: 255, B: 255, A: 160})

	line := color.NRGBA{R: 52, G: 80, B: 95, A: 170}
	c.roundedRect(364, 382, 636, 412, 15, line)
	c.roundedRect(364, 460, 636, 490, 15, line)
	c.roundedRect(364, 538, 554, 568, 15, line)

	proxy := color.NRGBA{R: 35, G: 186, B: 137, A: 255}
	proxyDark := color.NRGBA{R: 12, G: 89, B: 82, A: 255}
	c.strokeLine(258, 650, 426, 650, 28, proxy)
	c.strokeLine(598, 650, 766, 650, 28, proxy)
	c.circle(246, 650, 52, proxyDark)
	c.circle(246, 650, 32, proxy)
	c.circle(512, 650, 74, proxyDark)
	c.circle(512, 650, 50, proxy)
	c.circle(778, 650, 52, proxyDark)
	c.circle(778, 650, 32, proxy)
	c.poly([]point{{442, 600}, {442, 700}, {526, 650}}, color.NRGBA{R: 241, G: 248, B: 246, A: 255})
	c.poly([]point{{592, 610}, {592, 690}, {664, 650}}, color.NRGBA{R: 241, G: 248, B: 246, A: 255})

	return downsample(c.img, super)
}

func resizeImage(src *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := int((float64(x) + 0.5) * float64(srcW) / float64(size))
			sy := int((float64(y) + 0.5) * float64(srcH) / float64(size))
			if sx >= srcW {
				sx = srcW - 1
			}
			if sy >= srcH {
				sy = srcH - 1
			}
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

type point struct {
	x float64
	y float64
}

func (c *canvas) clear(clr color.Color) {
	draw.Draw(c.img, c.img.Bounds(), &image.Uniform{clr}, image.Point{}, draw.Src)
}

func (c *canvas) roundedRect(x, y, w, h, r float64, clr color.Color) {
	minX := int(x * c.scale)
	minY := int(y * c.scale)
	maxX := int((x + w) * c.scale)
	maxY := int((y + h) * c.scale)
	rr := r * c.scale
	for py := minY; py < maxY; py++ {
		for px := minX; px < maxX; px++ {
			fx := float64(px) + 0.5
			fy := float64(py) + 0.5
			cx := clamp(fx, float64(minX)+rr, float64(maxX)-rr)
			cy := clamp(fy, float64(minY)+rr, float64(maxY)-rr)
			if math.Hypot(fx-cx, fy-cy) <= rr {
				c.img.Set(px, py, clr)
			}
		}
	}
}

func (c *canvas) circle(x, y, r float64, clr color.Color) {
	cx := x * c.scale
	cy := y * c.scale
	rr := r * c.scale
	minX := int(cx - rr)
	maxX := int(cx + rr)
	minY := int(cy - rr)
	maxY := int(cy + rr)
	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			if px < 0 || py < 0 || px >= c.img.Bounds().Dx() || py >= c.img.Bounds().Dy() {
				continue
			}
			if math.Hypot(float64(px)+0.5-cx, float64(py)+0.5-cy) <= rr {
				c.img.Set(px, py, clr)
			}
		}
	}
}

func (c *canvas) strokeLine(x1, y1, x2, y2, width float64, clr color.Color) {
	x1 *= c.scale
	y1 *= c.scale
	x2 *= c.scale
	y2 *= c.scale
	r := width * c.scale / 2
	minX := int(math.Min(x1, x2) - r)
	maxX := int(math.Max(x1, x2) + r)
	minY := int(math.Min(y1, y2) - r)
	maxY := int(math.Max(y1, y2) + r)
	dx := x2 - x1
	dy := y2 - y1
	len2 := dx*dx + dy*dy
	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			if px < 0 || py < 0 || px >= c.img.Bounds().Dx() || py >= c.img.Bounds().Dy() {
				continue
			}
			t := ((float64(px)+0.5-x1)*dx + (float64(py)+0.5-y1)*dy) / len2
			t = clamp(t, 0, 1)
			nx := x1 + t*dx
			ny := y1 + t*dy
			if math.Hypot(float64(px)+0.5-nx, float64(py)+0.5-ny) <= r {
				c.img.Set(px, py, clr)
			}
		}
	}
}

func (c *canvas) poly(points []point, clr color.Color) {
	if len(points) < 3 {
		return
	}
	minX, maxX := points[0].x, points[0].x
	minY, maxY := points[0].y, points[0].y
	for _, p := range points[1:] {
		minX = math.Min(minX, p.x)
		maxX = math.Max(maxX, p.x)
		minY = math.Min(minY, p.y)
		maxY = math.Max(maxY, p.y)
	}
	for py := int(minY * c.scale); py <= int(maxY*c.scale); py++ {
		for px := int(minX * c.scale); px <= int(maxX*c.scale); px++ {
			if px < 0 || py < 0 || px >= c.img.Bounds().Dx() || py >= c.img.Bounds().Dy() {
				continue
			}
			if insidePoly((float64(px)+0.5)/c.scale, (float64(py)+0.5)/c.scale, points) {
				c.img.Set(px, py, clr)
			}
		}
	}
}

func insidePoly(x, y float64, points []point) bool {
	inside := false
	j := len(points) - 1
	for i := range points {
		pi := points[i]
		pj := points[j]
		if ((pi.y > y) != (pj.y > y)) && (x < (pj.x-pi.x)*(y-pi.y)/(pj.y-pi.y)+pi.x) {
			inside = !inside
		}
		j = i
	}
	return inside
}

func downsample(src *image.RGBA, factor int) *image.RGBA {
	w := src.Bounds().Dx() / factor
	h := src.Bounds().Dy() / factor
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	area := uint32(factor * factor)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a uint32
			for yy := 0; yy < factor; yy++ {
				for xx := 0; xx < factor; xx++ {
					cr, cg, cb, ca := src.At(x*factor+xx, y*factor+yy).RGBA()
					r += cr
					g += cg
					b += cb
					a += ca
				}
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8((r / area) >> 8),
				G: uint8((g / area) >> 8),
				B: uint8((b / area) >> 8),
				A: uint8((a / area) >> 8),
			})
		}
	}
	return dst
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()
	return png.Encode(fp, img)
}

func writeICO(path string, sizes []int) error {
	type entry struct {
		size int
		data []byte
	}
	entries := make([]entry, 0, len(sizes))
	for _, size := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, renderIcon(size)); err != nil {
			return err
		}
		entries = append(entries, entry{size: size, data: buf.Bytes()})
	}

	var out bytes.Buffer
	write16(&out, 0)
	write16(&out, 1)
	write16(&out, uint16(len(entries)))
	offset := 6 + len(entries)*16
	for _, entry := range entries {
		if entry.size >= 256 {
			out.WriteByte(0)
			out.WriteByte(0)
		} else {
			out.WriteByte(byte(entry.size))
			out.WriteByte(byte(entry.size))
		}
		out.WriteByte(0)
		out.WriteByte(0)
		write16(&out, 1)
		write16(&out, 32)
		write32(&out, uint32(len(entry.data)))
		write32(&out, uint32(offset))
		offset += len(entry.data)
	}
	for _, entry := range entries {
		out.Write(entry.data)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func writeICNS(path string) error {
	type chunk struct {
		code string
		size int
	}
	chunks := []chunk{
		{code: "icp4", size: 16},
		{code: "icp5", size: 32},
		{code: "icp6", size: 64},
		{code: "ic07", size: 128},
		{code: "ic08", size: 256},
		{code: "ic09", size: 512},
		{code: "ic10", size: 1024},
	}

	var payload bytes.Buffer
	for _, chunk := range chunks {
		var pngData bytes.Buffer
		if err := png.Encode(&pngData, renderIcon(chunk.size)); err != nil {
			return err
		}
		payload.WriteString(chunk.code)
		write32BE(&payload, uint32(8+pngData.Len()))
		payload.Write(pngData.Bytes())
	}

	var out bytes.Buffer
	out.WriteString("icns")
	write32BE(&out, uint32(8+payload.Len()))
	out.Write(payload.Bytes())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func write16(buf *bytes.Buffer, value uint16) {
	buf.WriteByte(byte(value))
	buf.WriteByte(byte(value >> 8))
}

func write32(buf *bytes.Buffer, value uint32) {
	buf.WriteByte(byte(value))
	buf.WriteByte(byte(value >> 8))
	buf.WriteByte(byte(value >> 16))
	buf.WriteByte(byte(value >> 24))
}

func write32BE(buf *bytes.Buffer, value uint32) {
	buf.WriteByte(byte(value >> 24))
	buf.WriteByte(byte(value >> 16))
	buf.WriteByte(byte(value >> 8))
	buf.WriteByte(byte(value))
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
