package utils

import (
	"math"
	"os"
)

func InLambda() bool {
	if lambdaTaskRoot := os.Getenv("LAMBDA_TASK_ROOT"); lambdaTaskRoot != "" {
		return true
	}
	return false
}

func MakeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}

func MakeRange10(min, max int) []int {
	diff := int(math.Abs(float64(min/10 - max/10)))
	a := make([]int, diff+1)
	for i := range a {
		a[i] = min + i*10
	}
	return a
}
