package main

import (
	"encoding/binary"
	"fmt"
	"github.com/racerxdl/segdsp/dsp/fft"
	"gopkg.in/alecthomas/kingpin.v2"
	"math"
	"math/rand"
	"os"
)

const amplitude = 0.05
const charPad = 1

var timeInterpFactor = 6000
var doVertical = false
var borderLength = 0

func GetCharDataVertical(char uint8) [][]float32 {
	const h = font16_10_height
	const cw = font16_10_charsPerLine

	cx := int(char & 0xF)
	cy := int((char&0xF0)>>4) * h

	outData := make([][]float32, font16_10_height)
	for i := 0; i < font16_10_height; i++ {
		outData[i] = make([]float32, font16_10_charWidth+charPad)
	}

	for l := 0; l < h; l++ {
		for b := uint16(0); b < font16_10_charWidth; b++ {
			v := font16_10[cy*cw+l*cw+cx]
			if uint16(v)&(1<<(7-b)) > 0 {
				outData[h-l-1][b] = amplitude
			}
		}
	}

	return outData
}

func GetCharData(char uint8) [][]float32 {
	const h = font16_10_height
	const cw = font16_10_charsPerLine

	cx := int(char & 0xF)
	cy := int((char&0xF0)>>4) * h

	outData := make([][]float32, font16_10_charWidth+charPad)

	for i := 0; i < font16_10_charWidth+charPad; i++ {
		outData[i] = make([]float32, font16_10_height+borderLength*2)
	}

	for b := uint16(0); b < font16_10_charWidth; b++ {
		for l := 0; l < h; l++ {
			v := font16_10[cy*cw+l*cw+cx]
			if uint16(v)&(1<<(7-b)) > 0 {
				outData[b][l+borderLength] = amplitude
			}
		}
	}

	return outData
}

func BuildCharBuffer(text string) (output [][]float32) {
	if doVertical {
		output = make([][]float32, font16_10_height)
		for i := 0; i < font16_10_height; i++ {
			output[i] = make([]float32, borderLength)
		}
		for i := 0; i < len(text); i++ {
			v := text[i]
			z := GetCharDataVertical(v)
			// Fill With Text Data
			for i, v := range z {
				output[i] = append(output[i], v...)
			}
		}
		for i := 0; i < font16_10_height; i++ {
			output[i] = append(output[i], make([]float32, borderLength)...)
		}
	} else {
		output = make([][]float32, 0)
		for i := 0; i < len(text); i++ {
			output = append(output, GetCharData(text[i])...)
		}
	}

	return output
}

var (
	text       = kingpin.Arg("text", "Text to paint").Required().String()
	sampleRate = kingpin.Flag("sampleRate", "Sample Rate (in sps)").Required().Int64()
	printSpeed = kingpin.Flag("printSpeed", "Print Speed in Chars / Second").Default("1").Float32()
	gain       = kingpin.Flag("gain", "Gain (in dB)").Default("0").Float32()
	vertical   = kingpin.Flag("vertical", "Print each character vertically").Default("false").Bool()
	outputName = kingpin.Flag("filename", "Name of the output file").Default("sample.cfile").String()
)

func main() {
	kingpin.Parse()

	speed := float32(1)
	doVertical = *vertical

	if printSpeed != nil {
		speed = *printSpeed
	}

	totalLength := 0
	totalHeight := 0

	if doVertical {
		fftSize := 64
		for fftSize < 16+len(*text)*(font16_10_charWidth+charPad) {
			fftSize *= 2
		}
		borderLength = (fftSize - len(*text)*(font16_10_charWidth+charPad)) / 2
		totalLength = borderLength*2 + len(*text)
		totalHeight = font16_10_height
	} else {
		borderLength = (64 - font16_10_charWidth - 4*charPad) / 2
		totalLength = borderLength*2 + font16_10_charWidth + 4*charPad
		totalHeight = font16_10_charWidth + charPad
	}

	buff := BuildCharBuffer(*text)
	iq := make([]complex64, 0)

	center := len(buff[0]) / 2

	fmt.Printf("File Output: %s\n", *outputName)
	fmt.Printf("Vertical Printing: %v\n", *vertical)
	fmt.Printf("Text: \"%s\"\n", *text)
	fmt.Printf("Text Length: %d\n", len(*text))
	fmt.Printf("Border Length: %d\n", borderLength)
	fmt.Printf("FFT Size: %d\n", len(buff[0]))
	fmt.Printf("Printing Speed: %f\n", speed)
	fmt.Printf("Sample Rate: %d SPS\n", *sampleRate)
	fmt.Printf("Total Length: %d\n", totalLength)
	fmt.Printf("Total Height: %d\n", totalHeight)
	fmt.Printf("Gain: %f dB\n", *gain)

	gainLinear := float32(math.Pow(10, float64((*gain)/20)))
	fmt.Printf("Linear Gain: %f\n", gainLinear)

	// Calculate Time Interp Factor
	// Since we have now how many samples each line has (which is len(buff[0])) we can calculate the duration
	// of each character line so it prints at right speed

	sampleDuration := 1.0 / float32(*sampleRate)

	charDuration := float32(totalHeight) / speed

	lineRate := float32(len(buff[0])) * sampleDuration

	fmt.Printf("Sample Duration: %f\n", sampleDuration)
	fmt.Printf("Line Rate: ~%d lines / second\n", int(1.0/lineRate))
	fmt.Printf("Char Duration: %f\n", charDuration)

	lineRate *= charDuration

	interpFactor := 1 / lineRate

	timeInterpFactor = int(interpFactor)

	fmt.Printf("Interpolation Factor: %d\n", timeInterpFactor)

	for i := 0; i < len(buff); i++ {
		// Add Borders to Line
		tbuff := buff[i]
		cT := make([]complex64, len(tbuff))

		// Repeat Several times to match line timing
		for n := 0; n < timeInterpFactor; n++ {
			for k := 0; k < len(tbuff); k++ {
				p := (center + k) % len(tbuff) // The FFT is symmetric so start from the center
				// Add Phase Noise to soften the peak a bit
				cT[k] = complex(tbuff[p], 0) * GenPhaseNoise()
			}

			// Calculate IFFT
			piq := fft.IFFT(cT)
			iq = append(iq, piq...)
		}
	}

	// Add some background noise
	for i := 0; i < len(iq); i++ {
		iq[i] *= complex(gainLinear, 0)
		iq[i] += complex((rand.Float32()-1)*(amplitude/15), 0)
	}

	// Write file output
	f, err := os.Create(*outputName)
	if err != nil {
		panic(err)
	}

	err = binary.Write(f, binary.LittleEndian, iq)
	if err != nil {
		panic(err)
	}

	_ = f.Close()
}
