package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/lyrical/src"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("failed to load .env file")
	}
	fmt.Println(os.Getenv("BOT_TOKEN"))
	src.InstallCmds(context.Background())
}
