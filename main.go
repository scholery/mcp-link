package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anyisalin/mcp-openapi-to-mcp-adapter/utils"
)

func main() {
	// 本地运行时执行这段代码
	runServer("0.0.0.0", 8089)

	// app := &cli.App{
	// 	Name:  "mcp-link",
	// 	Usage: "Convert OpenAPI to MCP compatible endpoints",
	// 	Commands: []*cli.Command{
	// 		{
	// 			Name:  "serve",
	// 			Usage: "Start the MCP Link server",
	// 			Flags: []cli.Flag{
	// 				&cli.IntFlag{
	// 					Name:    "port",
	// 					Aliases: []string{"p"},
	// 					Value:   8080,
	// 					Usage:   "Port to listen on",
	// 				},
	// 				&cli.StringFlag{
	// 					Name:    "host",
	// 					Aliases: []string{"H"},
	// 					Value:   "localhost",
	// 					Usage:   "Host to listen on",
	// 				},
	// 			},
	// 			Action: func(c *cli.Context) error {
	// 				return runServer(c.String("host"), c.Int("port"))
	// 			},
	// 		},
	// 	},
	// }

	// if err := app.Run(os.Args); err != nil {
	// 	log.Fatal(err)
	// }
}

func runServer(host string, port int) error {
	// Create server address
	addr := fmt.Sprintf("%s:%d", host, port)

	// Configure the SSE server
	ss := utils.NewSSEServer()

	// Create HTTP server with CORS middleware
	corsHandler := corsMiddleware(ss)
	server := &http.Server{
		Addr:    addr,
		Handler: corsHandler,
	}

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		fmt.Printf("Starting server on %s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	<-stop

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server
	fmt.Println("Shutting down server...")
	if err := ss.Shutdown(ctx); err != nil {
		log.Fatalf("Error shutting down SSE server: %v\n", err)
	}

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Error shutting down HTTP server: %v\n", err)
	}

	fmt.Println("Server gracefully stopped")
	return nil
}

// corsMiddleware adds CORS headers to allow requests from any origin
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Pass the request to the next handler
		next.ServeHTTP(w, r)
	})
}
