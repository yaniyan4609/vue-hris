package main

import (
	"encoding/json"
	"os"

	"github.com/SherClockHolmes/webpush-go"
)

func main() {
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"private": private, "public": public}); err != nil {
		os.Exit(1)
	}
}
