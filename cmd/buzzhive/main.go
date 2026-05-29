package main

import (
	"flag"
	"log"

	buzzhive "github.com/teatak/buzzhive/internal"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	adminDir := flag.String("admin-dir", "admin/dist", "built admin frontend directory")
	flag.Parse()

	if err := buzzhive.Run(*configPath, *adminDir); err != nil {
		log.Fatal(err)
	}
}
