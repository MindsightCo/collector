package main

import (
	"log"
)

func main() {
	config, err := ReadConfig()
	if err != nil {
		log.Fatal("error verifying config:", err)
	}

	log.Fatal(config.Loop())
}
