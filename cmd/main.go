package main

import (
	"log"
	"net"

	"github.com/YotoHana/tages-test-case/api"
	pb "github.com/YotoHana/tages-test-case/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)


func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	fileServer := api.New()
	pb.RegisterFileServiceServer(s, fileServer)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}