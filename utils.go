package main

import "fmt"

func VecToString(vec []float32) string {
	s := "["
	for i := range vec {
		s += fmt.Sprintf("%f", vec[i])
		if i < len(vec)-1 {
			s += ", "
		}
	}
	s += "]"

	return s
}
