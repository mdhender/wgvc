// Package aspectratio converts named or numeric aspect ratios into fixed-area dimensions.
package aspectratio

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const Default = "1:1"

// Dimensions returns width and height whose ratio matches value and whose area is one.
func Dimensions(value string) (width, height float64, err error) {
	if value == "" {
		value = Default
	}
	switch value {
	case "widescreen":
		value = "16:9"
	case "cinematic":
		value = "2.39:1"
	case "landscape":
		value = "4:3"
	case "portrait":
		value = "3:4"
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return 0, 0, fmt.Errorf("aspect ratio must be landscape, portrait, widescreen, cinematic, or use width:height notation: %q", value)
	}
	widthRatio, widthErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	heightRatio, heightErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if widthErr != nil || heightErr != nil || !(widthRatio > 0) || math.IsInf(widthRatio, 0) || !(heightRatio > 0) || math.IsInf(heightRatio, 0) {
		return 0, 0, fmt.Errorf("aspect ratio components must be finite and positive: %q", value)
	}
	ratio := widthRatio / heightRatio
	if math.IsInf(ratio, 0) || ratio < 1e-6 || ratio > 1e6 {
		return 0, 0, fmt.Errorf("aspect ratio must be between 1:1000000 and 1000000:1: %q", value)
	}
	return math.Sqrt(ratio), 1 / math.Sqrt(ratio), nil
}
