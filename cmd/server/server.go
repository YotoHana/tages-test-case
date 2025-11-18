package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/YotoHana/tages-test-case/api/proto"
	"github.com/YotoHana/tages-test-case/internal/api"
	"github.com/YotoHana/tages-test-case/internal/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	listenAddr = ":50051"
)

func main() {
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	streamLimiter := semaphore.NewSemaphore(10)
	unaryLimiter := semaphore.NewSemaphore(100)

	s := grpc.NewServer(
		grpc.ChainStreamInterceptor(semaphore.RateLimitStream(streamLimiter)),
		grpc.ChainUnaryInterceptor(semaphore.RateLimitUnary(unaryLimiter)),
	)
	fileServer, err := api.New()
	if err != nil {
		log.Fatalf("failed create new gRPC server: %v", err)
	}
	pb.RegisterFileServiceServer(s, fileServer)
	reflection.Register(s)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("gRPC Server is running on port %s\n", listenAddr)
		fmt.Println("Upload directory: ./uploads")
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop...")
		
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	<-sigChan
	
	fmt.Println()
	fmt.Println("Received shutdown signal...")
	fmt.Println("Waiting for active requests to complete...")

	s.GracefulStop()
	
	fmt.Println("Server stopped gracefully")
}