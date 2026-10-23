package subtitle

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MsToTimestamp converts milliseconds to "hh:mm:ss.mmm" format matching Node.js msToTimestamp.
func MsToTimestamp(ms int64) string {
	sign := ""
	if ms < 0 {
		sign = "-"
		ms = -ms
	}

	hours := ms / 3_600_000
	ms %= 3_600_000
	minutes := ms / 60_000
	ms %= 60_000
	seconds := ms / 1000
	millis := ms % 1000

	return fmt.Sprintf("%s%02d:%02d:%02d.%03d", sign, hours, minutes, seconds, millis)
}

// HHMMSSSToMs converts "HH:MM:SS.mmm", "MM:SS.mmm", or seconds strings to milliseconds.
func HHMMSSSToMs(str string) int64 {
	str = strings.TrimSpace(str)
	if str == "" {
		return 0
	}
	// Unify decimal separator (comma to dot, e.g. for SRT-style timestamps)
	s := strings.ReplaceAll(str, ",", ".")
	parts := strings.Split(s, ":")

	var h, m, sec float64
	if len(parts) == 3 {
		h, _ = strconv.ParseFloat(parts[0], 64)
		m, _ = strconv.ParseFloat(parts[1], 64)
		sec, _ = strconv.ParseFloat(parts[2], 64)
	} else if len(parts) == 2 {
		m, _ = strconv.ParseFloat(parts[0], 64)
		sec, _ = strconv.ParseFloat(parts[1], 64)
	} else {
		sec, _ = strconv.ParseFloat(s, 64)
	}

	totalMs := math.Round((h*3600 + m*60 + sec) * 1000)
	return int64(totalMs)
}

// SecondsToMs converts fractional seconds to rounded milliseconds.
func SecondsToMs(sec float64) int64 {
	return int64(math.Round(sec * 1000))
}

// MsToSeconds converts milliseconds to fractional seconds.
func MsToSeconds(ms int64) float64 {
	return float64(ms) / 1000.0
}
