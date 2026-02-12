package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func VecToString(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}

	var b strings.Builder
	b.Grow(len(vec) * 10) // Pre-allocate to reduce memory re-allocations
	b.WriteByte('[')

	for i, val := range vec {
		// 'f' for no exponent, -1 for smallest necessary digits, 32 for float32
		b.WriteString(strconv.FormatFloat(float64(val), 'f', -1, 32))
		if i < len(vec)-1 {
			b.WriteByte(',')
		}
	}

	b.WriteByte(']')
	return b.String()
}

func TimeFormat(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	hr := int(d.Hours())
	min := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hr, min, sec)
}
