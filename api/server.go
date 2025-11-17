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
	chunkSize = 64 * 1024
)

type Server struct {
	pb.UnimplementedFileServiceServer
	storage *storage.Storage
}
func (s *Server) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	items, err := s.storage.GetFileList()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read file directory: %v", err)
	}

	return &pb.ListResponse{Items: items}, nil
}

func (s *Server) Upload(stream pb.FileService_UploadServer) error {
	var id string
	var file *os.File

	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.UploadResponse{Id: id})
		}
		if err != nil {
			if file != nil {
				os.Remove(file.Name())
			}
			return status.Errorf(codes.Internal, "failed to receive data from client: %v", err)
		}

		if file == nil {
			if req.GetFilename() == "" {
				return status.Error(codes.InvalidArgument, "filename cannot be empty")
			}

			file, id, err = s.storage.CreateFile(req.GetFilename())

			if err != nil {
				return status.Errorf(codes.Internal, "failed to create file: %v", err)
			}
		}

		_, err = file.Write(req.GetChunk())
		if err != nil {
			os.Remove(file.Name())
			return status.Errorf(codes.Internal, "incomplete write file")
		}
	}

	
}

func (s *Server) Download(req *pb.DownloadRequest, stream pb.FileService_DownloadServer) error {
	fileID := req.GetId()

	if fileID == "" {
		return status.Error(codes.InvalidArgument, "id cannot be empty")
	}

	fullPath, originalName, err := s.storage.FindFileByID(fileID)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Errorf(codes.NotFound, "file with id '%s' not found", fileID)
		}

		return status.Errorf(codes.Internal, "failed to locate file: %v", err)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Errorf(codes.NotFound, "file not found: %v", err)
		}

		return status.Errorf(codes.Internal, "failed to open file: %v", err)
	}
	defer file.Close()

	err = stream.Send(&pb.DownloadResponse{
		Payload: &pb.DownloadResponse_Info{
			Info: &pb.FileInfo{Name: originalName},
		},
	})
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to send file metadata: %v", err)
	}

	buf := make([]byte, chunkSize)

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}

		if err != nil {
			return status.Errorf(codes.Internal, "failed to read file: %v", err)
		}

		if n > 0 {
			stream.Send(&pb.DownloadResponse{
				Payload: &pb.DownloadResponse_Chunk{
					Chunk: buf[:n],
				},
			})
		}
	}

	return nil
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