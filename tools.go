package main

import (
	"github.com/racerxdl/segdsp/tools"
	"math"
	"math/rand"
)

var r = float32(math.Exp(0))

func GenPhaseNoise() complex64 {
	v := rand.Float32() * 2 * math.Pi
	s := tools.Sin(v)
	c := tools.Cos(v)
	return complex(r*c, r*s)
}
