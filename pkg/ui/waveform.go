package ui

import (
	"image"
	"image/color"
	"log"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// WaveformVisualizer is a custom widget that displays a waveform visualization
type WaveformVisualizer struct {
	widget.BaseWidget
	amplitude    float32
	waveColor    color.Color
	levels       []float32 // Store current audio levels
	barCount     int       // Number of bars to display
	mu           sync.Mutex
	animating    bool
	lastUpdate   time.Time
	renderObject *canvas.Raster
}

// NewWaveformVisualizer creates a new waveform visualizer widget
func NewWaveformVisualizer(color color.Color) *WaveformVisualizer {
	w := &WaveformVisualizer{
		amplitude:  0.5,
		waveColor:  color,
		levels:     make([]float32, 20), // Use fewer bars for a cleaner look
		barCount:   20,
		animating:  false,
		lastUpdate: time.Now(),
	}
	w.ExtendBaseWidget(w)
	return w
}

// SetAmplitude updates the waveform amplitude and triggers a refresh
func (w *WaveformVisualizer) SetAmplitude(amplitude float32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Ensure amplitude is between 0 and 1
	if amplitude < 0 {
		amplitude = 0
	} else if amplitude > 1 {
		amplitude = 1
	}

	// Update amplitude with smoother transition
	// Blend the new amplitude with the previous one for smoother changes
	prevAmplitude := w.amplitude
	w.amplitude = prevAmplitude*0.7 + amplitude*0.3 // Weighted average for smoother transitions

	// Update the levels with varying heights based on amplitude and position
	for i := 0; i < w.barCount; i++ {
		// Create a soft sinusoidal wave pattern across the bars
		// Bars in middle are taller, bars at edges are shorter
		position := float64(i) / float64(w.barCount-1)
		centerDistance := math.Abs(position-0.5) * 2.0 // 0 at center, 1 at edges

		// Calculate target level with sinusoidal falloff from center
		falloff := 1.0 - (centerDistance * 0.7)

		// Blend current level with target level for smooth transitions
		targetLevel := float32(falloff) * w.amplitude
		w.levels[i] = w.levels[i]*0.8 + targetLevel*0.2
	}

	// Request a repaint
	canvas.Refresh(w)
}

// StartListening begins the animation loop for the waveform
func (w *WaveformVisualizer) StartListening() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.animating {
		w.animating = true
		w.lastUpdate = time.Now()
		go w.startAnimation()
	}
}

// StopListening stops the animation loop
func (w *WaveformVisualizer) StopListening() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Only change state if actually animating to prevent double-stops
	if w.animating {
		w.animating = false
		// Log the state change for debugging
		log.Println("Waveform animation stopped by explicit call")
	}
}

// CreateRenderer implements the widget interface
func (w *WaveformVisualizer) CreateRenderer() fyne.WidgetRenderer {
	w.renderObject = canvas.NewRaster(w.drawWaveform)

	renderer := &waveformRenderer{
		waveform: w,
		objects:  []fyne.CanvasObject{w.renderObject},
	}
	return renderer
}

// MinSize implements the Widget interface - make sure it takes up enough space
func (w *WaveformVisualizer) MinSize() fyne.Size {
	// Return a reasonable minimum size that ensures visibility
	return fyne.NewSize(400, 60)
}

// startAnimation is the animation loop for the waveform
func (w *WaveformVisualizer) startAnimation() {
	// Add panic recovery to prevent application crashes
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic in waveform animation:", r)
		}
	}()

	for {
		// Check if we should continue animating
		w.mu.Lock()
		if !w.animating {
			w.mu.Unlock()
			break
		}

		// Calculate the time delta for smooth animation
		now := time.Now()
		w.lastUpdate = now

		// In idle mode, create gentle wave-like movements
		if w.amplitude < 0.1 {
			// Create soft pulsing effect
			for i := 0; i < w.barCount; i++ {
				// Create a gentle idle animation based on time and position
				position := float64(i) / float64(w.barCount-1)
				timeOffset := float64(now.UnixNano()%2000000000) / 2000000000.0

				// Varied frequency for each bar
				frequency := 2.0 + position*2.0

				// Calculate idle level (gentle waves)
				idleLevel := 0.05 + 0.03*math.Sin(frequency*math.Pi*(position+timeOffset))

				// Smooth transition
				w.levels[i] = w.levels[i]*0.9 + float32(idleLevel)*0.1
			}
		}

		// Release lock before requesting refresh
		w.mu.Unlock()

		// Request repaint and wait for next frame
		canvas.Refresh(w)
		time.Sleep(time.Second / 10) // Reduced from 20 FPS to 10 FPS for significantly less CPU usage
	}
}

// drawWaveform renders the actual waveform visualization
func (w *WaveformVisualizer) drawWaveform(width, height int) image.Image {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Create a new RGBA image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Convert the wave color to RGBA
	r, g, b, a := w.waveColor.RGBA()
	waveColorRGBA := color.RGBA{
		uint8(r >> 8),
		uint8(g >> 8),
		uint8(b >> 8),
		uint8(a >> 8),
	}

	// Calculate center line
	centerY := height / 2

	// Draw center line with reduced opacity
	baselineColor := color.RGBA{
		uint8(r >> 9), // Reduced intensity
		uint8(g >> 9),
		uint8(b >> 9),
		uint8(a >> 9),
	}
	for x := 0; x < width; x++ {
		img.Set(x, centerY, baselineColor)
	}

	// Calculate bar width and spacing
	// Reduce bar count when window is narrow to avoid excessive drawing operations
	effectiveBarCount := w.barCount
	if width < 100 {
		effectiveBarCount = w.barCount / 2
	}

	barWidth := (width - effectiveBarCount) / effectiveBarCount
	if barWidth < 2 {
		barWidth = 2 // Minimum bar width
	}

	spacing := (width - (barWidth * effectiveBarCount)) / (effectiveBarCount + 1)
	if spacing < 1 {
		spacing = 1
	}

	// Draw equalizer bars (mirrored top and bottom)
	for i := 0; i < effectiveBarCount; i++ {
		// Skip drawing every other bar if width is very small to improve performance
		if width < 50 && i%2 == 1 {
			continue
		}

		// Ensure index is in bounds
		levelIndex := (i * w.barCount) / effectiveBarCount
		if levelIndex >= len(w.levels) {
			levelIndex = len(w.levels) - 1
		}

		level := w.levels[levelIndex]

		// Calculate bar position
		x := spacing + i*(barWidth+spacing)

		// Calculate bar height (scaled to half-height, mirrored)
		barHeight := int(level * float32(height) * 0.45)

		// Skip drawing very small bars to improve performance
		if barHeight < 2 {
			continue
		}

		// Draw top bar (going up from center)
		for y := centerY; y >= centerY-barHeight; y-- {
			// Draw bar with gradient effect (brighter at edges)
			barColor := adjustBrightness(waveColorRGBA,
				1.0-float64(centerY-y)/float64(barHeight+1))

			for bx := 0; bx < barWidth; bx++ {
				if x+bx < width {
					img.Set(x+bx, y, barColor)
				}
			}
		}

		// Draw bottom bar (going down from center, mirror of top)
		for y := centerY; y <= centerY+barHeight; y++ {
			// Draw bar with gradient effect (brighter at edges)
			barColor := adjustBrightness(waveColorRGBA,
				1.0-float64(y-centerY)/float64(barHeight+1))

			for bx := 0; bx < barWidth; bx++ {
				if x+bx < width {
					img.Set(x+bx, y, barColor)
				}
			}
		}
	}

	return img
}

// adjustBrightness adjusts the brightness of a color
func adjustBrightness(c color.RGBA, factor float64) color.RGBA {
	// Ensure factor is between 0.2 and 1.0
	if factor < 0.2 {
		factor = 0.2
	} else if factor > 1.0 {
		factor = 1.0
	}

	return color.RGBA{
		uint8(float64(c.R) * factor),
		uint8(float64(c.G) * factor),
		uint8(float64(c.B) * factor),
		c.A,
	}
}

// waveformRenderer implements the fyne.WidgetRenderer interface
type waveformRenderer struct {
	waveform *WaveformVisualizer
	objects  []fyne.CanvasObject
}

// Layout positions the waveform display
func (r *waveformRenderer) Layout(size fyne.Size) {
	r.waveform.renderObject.Resize(size)
}

// MinSize returns the minimum size for the waveform
func (r *waveformRenderer) MinSize() fyne.Size {
	return fyne.NewSize(100, 40)
}

// Objects returns the objects this renderer manages
func (r *waveformRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Refresh causes this renderer to be redrawn
func (r *waveformRenderer) Refresh() {
	canvas.Refresh(r.waveform.renderObject)
}

// Destroy releases any resources associated with this renderer
func (r *waveformRenderer) Destroy() {}
