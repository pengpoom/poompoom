package imaging

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	DefaultGenerateSize   = "1024x1024"
	maxGenerateEdge       = 3840
	maxGenerateRatio      = 3
	minGeneratePixels     = 655360
	maxGeneratePixels     = 8294400
	maxFreeGeneratePixels = 1577536
	sizeMultiple          = 16
)

var commonGenerateSizes = []string{
	DefaultGenerateSize,
	"1536x1024",
	"1024x1536",
	"3840x2160",
	"2160x3840",
}

func NormalizeGenerateSize(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	if trimmed == "" || trimmed == "auto" {
		return ""
	}
	return trimmed
}

func ValidateGenerateSize(value string) error {
	normalized := NormalizeGenerateSize(value)
	if normalized == "" {
		return nil
	}

	width, height, ok := parseGenerateSize(normalized)
	if !ok {
		return fmt.Errorf("invalid size %q: expected WIDTHxHEIGHT", value)
	}
	if width%sizeMultiple != 0 || height%sizeMultiple != 0 {
		return fmt.Errorf("invalid size %q: width and height must be multiples of %d", normalized, sizeMultiple)
	}

	longest, shortest := width, height
	if shortest > longest {
		longest, shortest = shortest, longest
	}
	if longest > maxGenerateEdge {
		return fmt.Errorf("invalid size %q: the longest edge must be less than or equal to %d", normalized, maxGenerateEdge)
	}
	if shortest == 0 || longest > shortest*maxGenerateRatio {
		return fmt.Errorf("invalid size %q: aspect ratio must not exceed %d:1", normalized, maxGenerateRatio)
	}

	pixels := width * height
	if pixels < minGeneratePixels || pixels > maxGeneratePixels {
		return fmt.Errorf("invalid size %q: total pixels must be between %d and %d", normalized, minGeneratePixels, maxGeneratePixels)
	}
	return nil
}

func IsSupportedGenerateSize(value string) bool {
	normalized := NormalizeGenerateSize(value)
	return normalized != "" && ValidateGenerateSize(normalized) == nil
}

func SupportedGenerateSizes() []string {
	items := make([]string, len(commonGenerateSizes))
	copy(items, commonGenerateSizes)
	return items
}

func RequiresPaidGenerateAccount(value string) bool {
	normalized := NormalizeGenerateSize(value)
	if normalized == "" {
		return false
	}
	width, height, ok := parseGenerateSize(normalized)
	if !ok {
		return false
	}
	return width*height > maxFreeGeneratePixels
}

func parseGenerateSize(value string) (int, int, bool) {
	widthRaw, heightRaw, ok := strings.Cut(value, "x")
	if !ok {
		return 0, 0, false
	}
	width, err := strconv.Atoi(widthRaw)
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(heightRaw)
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
