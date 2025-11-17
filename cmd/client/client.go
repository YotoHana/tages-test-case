package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	pb "github.com/YotoHana/tages-test-case/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	serverAddr = "localhost:50051"
	chunkSize = 64 * 1024
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println(" client upload <filepath>")
		fmt.Println(" client download <file_id> <output_path>")
		fmt.Println(" client list")
		os.Exit(1)
	}

	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect: %v", err))
	}
	defer conn.Close()

	client := pb.NewFileServiceClient(conn)

	command := os.Args[1]

	switch command {
	case "upload":
		if len(os.Args) < 3 {
			fmt.Println("Usage: client upload <filepath>")
			os.Exit(1)
		}

		uploadFile(client, os.Args[2])

	case "download":
		if len(os.Args) < 4 {
			fmt.Println("Usage: client download <file_id> <output_path>")
			os.Exit(1)
		}

		downloadFile(client, os.Args[2], os.Args[3])

	case "list":
		listFile(client)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func uploadFile(client pb.FileServiceClient, path string) {
	file, err := os.Open(path)
	if err != nil {
		panic(fmt.Sprintf("Failed to open file: %v", err))
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		panic(fmt.Sprintf("Failed to get file info: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
	defer cancel()

	stream, err := client.Upload(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to create stream: %v", err))
	}

	err = stream.Send(&pb.UploadRequest{
		Data: &pb.UploadRequest_Filename{
			Filename: filepath.Base(path),
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to send file name: %v", err))
	}

	buffer := make([]byte, chunkSize)
	totalSent := 0

	fmt.Printf("Uploading %s (%d bytes)...\n", filepath.Base(path), fileInfo.Size())

	for {
		n, err := file.Read(buffer)
		
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(fmt.Sprintf("Failed to read file: %v", err))
		}

		err = stream.Send(&pb.UploadRequest{
			Data: &pb.UploadRequest_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			panic(fmt.Sprintf("Failed to send chunk: %v", err))
		}

		totalSent += n
		progress := float64(totalSent) / float64(fileInfo.Size()) * 100
		fmt.Printf("\rProgress: %.1f%%", progress)
	}

	fmt.Println()

	response, err := stream.CloseAndRecv()
	if err != nil {
		panic(fmt.Sprintf("Failed to receive response: %v", err))
	}

	fmt.Printf("Upload successful!\n")
	fmt.Printf("File ID: %s\n", response.Id)
}

func downloadFile(client pb.FileServiceClient, fileID string, outputPath string) {
	var file *os.File

	err := os.MkdirAll(outputPath, 0755)
	if err != nil {
		panic(fmt.Sprintf("Failed to create directory: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
	defer cancel()
	
	stream, err := client.Download(ctx, &pb.DownloadRequest{Id: fileID})
	if err != nil {
		panic(fmt.Sprintf("Failed to create stream: %v", err))
	}

	for {
		resp, err := stream.Recv()
		
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(fmt.Sprintf("Failed to get response: %v", err))
		}

		if file == nil {
			file, err = os.Create(filepath.Join(outputPath, resp.GetInfo().GetName()))
			if err != nil {
				panic(fmt.Sprintf("Failed to create file: %v", err))
			}
		}

		file.Write(resp.GetChunk())
	}
}

func listFile(client pb.FileServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
	defer cancel()

	resp, err := client.List(ctx, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create response: %v", err))
	}

	items := resp.GetItems()

	fmt.Println("List Files:")

	for _, item := range items {
		fmt.Printf(
			"ID: %v | FileName: %v | Created_At: %v | Updated_At: %v \n",
			item.Id,
			item.Name,
			item.CreatedAt.AsTime(),
			item.UpdatedAt.AsTime(),
		)
	}
}