package fuzz

import "github.com/andybalholm/cascadia"

// Fuzz is the entrypoint used by the go-fuzz framework
func Fuzz(data []byte) int {
	_, err := cascadia.Compile(string(data))
	if err != nil {
		return 0
	}
	return 1
}
