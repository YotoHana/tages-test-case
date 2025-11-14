package api

import (
	"context"
	"io"
	"os"

	pb "github.com/YotoHana/tages-test-case/api/proto"
	"github.com/YotoHana/tages-test-case/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ErrUploadFile = "failed upload file"
)

type Server struct {
	pb.UnimplementedFileServiceServer
	storage *storage.Storage
}
func (s *Server) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	return &pb.ListResponse{Items: []*pb.ListResponse_Item{}}, nil
}

func (s *Server) Upload(stream pb.FileService_UploadServer) error {
	var id string
	var file *os.File

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.UploadResponse{Id: id})
		}
		if err != nil {
			return status.Errorf(codes.Internal, ErrUploadFile)
		}

		if file == nil {
			file, id, err = s.storage.CreateFile(req.GetFilename())
			if err != nil {
				return status.Errorf(codes.Internal, ErrUploadFile)
			}
		}

		file.Write(req.GetChunk())
	}

	
}

func (s *Server) Download(_ *pb.DownloadRequest, stream pb.FileService_DownloadServer) error {
	return status.Errorf(codes.Unimplemented, "download not implemented yet")
}

func New() (*Server, error) {
	storage, err := storage.New()
	if err != nil {
		return nil, err
	}
	
	server := &Server{
		storage: storage,
	}

	return server, nil
}