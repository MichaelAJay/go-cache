//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/MichaelAJay/go-cache/internal/testenv"
)

// validate-services.go - Validate docker-compose services are ready
func main() {
	ctx := context.Background()
	
	validator := &testenv.ServiceValidator{
		RedisAddr:    "localhost:6379",
		ToxiproxyAPI: "http://localhost:8474",
	}

	fmt.Println("✅ Validating docker-compose services...")

	if err := validator.ValidateServices(ctx, testenv.ModeCompose); err != nil {
		fmt.Printf("❌ Service validation failed: %v\n", err)
		fmt.Println("\n💡 To fix this:")
		fmt.Println("   1. Ensure docker-compose is running: docker-compose up -d")
		fmt.Println("   2. Check service health: docker-compose ps")
		fmt.Println("   3. Check logs: docker-compose logs")
		fmt.Println("   4. Try restarting: docker-compose restart")
		os.Exit(1)
	}

	fmt.Println("✅ All services are ready and accessible")
	fmt.Println("   - Redis: Connected and responsive")
	fmt.Println("   - Latency testing: Available via toxiproxy if enabled")
}