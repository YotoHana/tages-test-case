package api

import (
	"context"

	pb "github.com/YotoHana/tages-test-case/api/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedFileServiceServer
}

func (s *Server) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	return &pb.ListResponse{Items: []*pb.ListResponse_Item{}}, nil
}

func (s *Server) Upload(stream pb.FileService_UploadServer) error {
	return status.Errorf(codes.Unimplemented, "upload not implemented yet")
}

func (s *Server) Download(_ *pb.DownloadRequest, stream pb.FileService_DownloadServer) error {
	return status.Errorf(codes.Unimplemented, "download not implemented yet")
}

func New() *Server {
	return &Server{
		pb.UnimplementedFileServiceServer{},
	}
}