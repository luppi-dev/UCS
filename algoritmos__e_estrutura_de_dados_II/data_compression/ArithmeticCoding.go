package main

import (
	"encoding/gob"
	"fmt"
	"os"
	"strings"
)

type EncodedData struct {
	Code float64
	Odds map[rune]float64
}

func CalcCharsOdds(text string) map[rune]float64 {
	frequencies := make(map[rune]int)
	total := len(text)

	for _, char := range text {
		frequencies[char]++
	}

	odds := make(map[rune]float64)
	for char, freq := range frequencies {
		odds[char] = float64(freq) / float64(total)
	}

	return odds
}

func CalcInterval(odds map[rune]float64, char rune) [2]float64 {
	low := 0.0
	for c, odd := range odds {
		if c == char {
			return [2]float64{low, low + odd}
		}
		low += odd
	}
	return [2]float64{0, 0}
}

func Encode(text string, odds map[rune]float64) (float64, float64) {
	low, high := 0.0, 1.0

	for _, char := range text {
		rangeWidth := high - low
		lowHigh := CalcInterval(odds, char)
		high = low + rangeWidth*lowHigh[1]
		low = low + rangeWidth*lowHigh[0]
	}

	return low, high
}

func Decode(data EncodedData, size int) string {
	result := ""
	code := data.Code
	odds := data.Odds

	for i := 0; i < size; i++ {
		for char := range odds {
			lowHigh := CalcInterval(odds, char)
			if code >= lowHigh[0] && code < lowHigh[1] {
				result += string(char)
				code = (code - lowHigh[0]) / (lowHigh[1] - lowHigh[0])
				break
			}
		}
	}

	return result
}

func SaveEncodedData(path string, data EncodedData) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

func readTextFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
func readEncodedDataFile(path string) (EncodedData, error) {
	var data EncodedData
	file, err := os.Open(path)
	if err != nil {
		return data, err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return data, err
	}

	return data, nil
}

func main() {
	text, err := readTextFile("myText.txt")
	if err != nil {
		fmt.Printf("Error reading text file: %v\n", err)
		return
	}
	text = strings.TrimSpace(text)

	odds := CalcCharsOdds(text)

	low, high := Encode(text, odds)
	fmt.Printf("Encoded interval: [%.10f, %.10f]\n", low, high)

	code := (low + high) / 2
	data := EncodedData{
		Code: code,
		Odds: odds,
	}

	err = SaveEncodedData("encoded.gob", data)
	if err != nil {
		fmt.Printf("Error writing encoded file: %v\n", err)
		return
	}

	readedData, err := readEncodedDataFile("encoded.gob")
	if err != nil {
		fmt.Printf("Error reading encoded file: %v\n", err)
		return
	}

	decodedText := Decode(readedData, len(text))
	fmt.Printf("Decoded text: %s\n", decodedText)
}
