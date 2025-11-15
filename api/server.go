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
	ErrUploadFile = "Failed upload file"
	ErrListFiles = "Failed list files"
	ErrDownloadFile = "Failed download file"
	ErrFileNotFound = "File not found"
	chunkSize = 64 * 1024
)

type Server struct {
	pb.UnimplementedFileServiceServer
	storage *storage.Storage
}
func (s *Server) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	items, err := s.storage.GetFileList()
	if err != nil {
		return &pb.ListResponse{}, status.Errorf(codes.NotFound, ErrFileNotFound)
	}

	return &pb.ListResponse{Items: items}, nil
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

func (s *Server) Download(req *pb.DownloadRequest, stream pb.FileService_DownloadServer) error {
	fullPath, originalName, err := s.storage.FindFileByID(req.GetId())
	if err != nil {
		return status.Error(codes.Internal, ErrDownloadFile)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return status.Error(codes.Internal, ErrDownloadFile)
	}
	defer file.Close()

	err = stream.Send(&pb.DownloadResponse{
		Payload: &pb.DownloadResponse_Info{
			Info: &pb.FileInfo{Name: originalName},
		},
	})
	if err != nil {
		return status.Error(codes.Internal, ErrDownloadFile)
	}

	buf := make([]byte, chunkSize)

	for {
		n, err := file.Read(buf)
		if err != nil {
			return status.Error(codes.Internal, ErrDownloadFile)
		}

		if n > 0 {
			stream.Send(&pb.DownloadResponse{
				Payload: &pb.DownloadResponse_Chunk{
					Chunk: buf[:n],
				},
			})
		}
	}

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