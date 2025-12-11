package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

type FingerprintGenerator interface {
	GenerateFingerprint() (map[string]interface{}, error)
	GenerateCanvas() string
	GenerateWebGL() map[string]interface{}
	GenerateAudio() map[string]interface{}
}

type fingerprintGenerator struct{}

func NewFingerprintGenerator() FingerprintGenerator {
	return &fingerprintGenerator{}
}

func (f *fingerprintGenerator) GenerateFingerprint() (map[string]interface{}, error) {
	fingerprint := map[string]interface{}{
		"canvas":           f.GenerateCanvas(),
		"webgl":           f.GenerateWebGL(),
		"audio":           f.GenerateAudio(),
		"fonts":           f.generateFonts(),
		"screen":          f.generateScreen(),
		"timezone":        f.generateTimezone(),
		"cpu_cores":       f.generateCPUCores(),
		"memory":          f.generateMemory(),
		"platform":        f.generatePlatform(),
		"touch_support":   f.generateTouchSupport(),
		"connection_type": f.generateConnectionType(),
		"do_not_track":    f.generateDoNotTrack(),
		"color_depth":     f.generateColorDepth(),
		"pixel_ratio":     f.generatePixelRatio(),
		"session_storage": true,
		"local_storage":   true,
		"indexed_db":      true,
		"webrtc_id":       f.generateWebRTCId(),
	}

	return fingerprint, nil
}

func (f *fingerprintGenerator) GenerateCanvas() string {
	// Generate unique but consistent canvas fingerprint
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (f *fingerprintGenerator) GenerateWebGL() map[string]interface{} {
	vendors := []string{
		"Intel Inc.",
		"NVIDIA Corporation",
		"AMD",
		"Apple Inc.",
		"Qualcomm",
	}

	renderers := []string{
		"Intel Iris OpenGL Engine",
		"NVIDIA GeForce GTX 1060",
		"AMD Radeon Pro 5300M",
		"Apple M1",
		"Adreno (TM) 640",
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(vendors))))
	vendorIndex := int(n.Int64())

	return map[string]interface{}{
		"vendor":                vendors[vendorIndex],
		"renderer":              renderers[vendorIndex],
		"unmasked_vendor":       vendors[vendorIndex],
		"unmasked_renderer":     renderers[vendorIndex],
		"version":               "WebGL 2.0",
		"shading_language":      "WebGL GLSL ES 3.00",
		"max_texture_size":      16384,
		"max_viewport":          []int{16384, 16384},
		"max_cube_map_texture":  16384,
		"max_render_buffer":     16384,
		"max_vertex_attribs":    16,
		"max_vertex_texture":    16,
		"max_combined_texture":  80,
		"max_fragment_uniform":  4096,
		"max_vertex_uniform":    4096,
		"aliased_line_width":    []float32{1, 10},
		"aliased_point_size":    []float32{1, 2048},
		"depth_bits":            24,
		"stencil_bits":          8,
		"antialias":            true,
		"angle":                "ANGLE (Intel, Intel Iris OpenGL Engine, OpenGL 4.1)",
	}
}

func (f *fingerprintGenerator) GenerateAudio() map[string]interface{} {
	sampleRates := []int{44100, 48000}
	n, _ := rand.Int(rand.Reader, big.NewInt(2))

	return map[string]interface{}{
		"sample_rate":       sampleRates[n.Int64()],
		"channels":          2,
		"max_channels":      8,
		"channel_count":     2,
		"latency":           0.01 + float64(time.Now().UnixNano()%100)/10000,
		"state":             "running",
		"destination_type":  "speakers",
		"oscillator_type":   "sine",
		"compressor_ratio":  12,
		"compressor_attack": 0.003,
	}
}

func (f *fingerprintGenerator) generateFonts() []string {
	baseFonts := []string{
		"Arial", "Verdana", "Times New Roman", "Courier New",
		"Georgia", "Trebuchet MS", "Comic Sans MS", "Impact",
		"Helvetica", "Tahoma", "Segoe UI", "Calibri",
	}

	// Randomize font order and selection
	n, _ := rand.Int(rand.Reader, big.NewInt(4))
	extraFonts := int(n.Int64()) + 8

	selectedFonts := make([]string, 0, extraFonts)
	selectedFonts = append(selectedFonts, baseFonts...)

	return selectedFonts
}

func (f *fingerprintGenerator) generateScreen() map[string]interface{} {
	resolutions := [][]int{
		{1920, 1080}, {2560, 1440}, {1366, 768},
		{1440, 900}, {1536, 864}, {1680, 1050},
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(resolutions))))
	res := resolutions[n.Int64()]

	return map[string]interface{}{
		"width":            res[0],
		"height":           res[1],
		"available_width":  res[0],
		"available_height": res[1] - 40, // Taskbar height
		"color_depth":      24,
		"pixel_depth":      24,
	}
}

func (f *fingerprintGenerator) generateTimezone() string {
	timezones := []string{
		"America/New_York",
		"America/Chicago",
		"America/Denver",
		"America/Los_Angeles",
		"Europe/London",
		"Europe/Paris",
		"Asia/Tokyo",
		"Australia/Sydney",
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(timezones))))
	return timezones[n.Int64()]
}

func (f *fingerprintGenerator) generateCPUCores() int {
	cores := []int{2, 4, 6, 8, 12, 16}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(cores))))
	return cores[n.Int64()]
}

func (f *fingerprintGenerator) generateMemory() int {
	memory := []int{4, 8, 16, 32}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(memory))))
	return memory[n.Int64()]
}

func (f *fingerprintGenerator) generatePlatform() string {
	platforms := []string{"Win32", "MacIntel", "Linux x86_64"}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(platforms))))
	return platforms[n.Int64()]
}

func (f *fingerprintGenerator) generateTouchSupport() map[string]interface{} {
	n, _ := rand.Int(rand.Reader, big.NewInt(100))
	hasTouch := n.Int64() < 30 // 30% chance of touch support

	if hasTouch {
		return map[string]interface{}{
			"max_touch_points": 10,
			"has_touch":        true,
			"touch_event":      true,
		}
	}

	return map[string]interface{}{
		"max_touch_points": 0,
		"has_touch":        false,
		"touch_event":      false,
	}
}

func (f *fingerprintGenerator) generateConnectionType() string {
	types := []string{"wifi", "ethernet", "4g"}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(types))))
	return types[n.Int64()]
}

func (f *fingerprintGenerator) generateDoNotTrack() interface{} {
	n, _ := rand.Int(rand.Reader, big.NewInt(100))
	if n.Int64() < 70 {
		return nil
	}
	return "1"
}

func (f *fingerprintGenerator) generateColorDepth() int {
	depths := []int{24, 32}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(depths))))
	return depths[n.Int64()]
}

func (f *fingerprintGenerator) generatePixelRatio() float64 {
	ratios := []float64{1, 1.25, 1.5, 2, 2.5}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(ratios))))
	return ratios[n.Int64()]
}

func (f *fingerprintGenerator) generateWebRTCId() string {
	// Generate random WebRTC identifier
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return fmt.Sprintf("%x-%x-%x-%x", bytes[0:8], bytes[8:12], bytes[12:16], bytes[16:32])
}