//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MichaelAJay/go-cache/internal/testenv"
)

// wait-for-services.go - Wait for docker-compose services to be ready
func main() {
	ctx := context.Background()

	validator := &testenv.ServiceValidator{
		RedisAddr:    "localhost:6379",
		ToxiproxyAPI: "http://localhost:8474",
	}

	fmt.Println("⏳ Waiting for docker-compose services to be ready...")

	// Wait up to 60 seconds for services
	for i := range 60 {
		if err := validator.ValidateServices(ctx, testenv.ModeCompose); err == nil {
			fmt.Println("✅ All services ready")
			return
		}

		// Show progress every 10 seconds
		if i%10 == 0 && i > 0 {
			fmt.Printf("⏳ Still waiting... (%ds elapsed)\n", i)
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Println("❌ Services failed to start within 60 seconds")
	fmt.Println("\n💡 To fix this:")
	fmt.Println("   1. Check services: docker-compose ps")
	fmt.Println("   2. Check logs: docker-compose logs")
	fmt.Println("   3. Try restarting: docker-compose restart")
	os.Exit(1)
}