package main

import (
	"log"
	"net"

	"github.com/YotoHana/tages-test-case/internal/api"
	pb "github.com/YotoHana/tages-test-case/api/proto"
	"github.com/YotoHana/tages-test-case/internal/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)


func main() {
	lis, err := net.Listen("tcp", ":50051")
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
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}