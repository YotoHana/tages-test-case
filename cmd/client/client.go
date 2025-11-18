package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	pb "github.com/YotoHana/tages-test-case/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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
		fmt.Println(" client test-limits")
		os.Exit(1)
	}

	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		os.Exit(1)
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
	
	case "test-limits":
		testRateLimits()

	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func testRateLimits() {
	fmt.Println("Testing rate limits...")

	testFile := createTestFile()
	defer os.Remove(testFile)

	fmt.Println("Test 1: Upload rate limit (max 10 concurrent)")
	fmt.Println("Sending 15 concurrent upload requests...")

	testUploadLimit(testFile, 15)

	time.Sleep(time.Second * 2)

	fmt.Println("Test 2: List rate limit (max 100 concurrent)")
	fmt.Println("Sending 105 concurrent list requests...")

	testListLimit(105)

	fmt.Println("Rate limit testing complete!")
}

func createTestFile() string {
	tmpFile, err := os.CreateTemp("", "test_upload_*.dat")
	if err != nil {
		panic(err)
	}

	data := make([]byte, 20 * 1024 * 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	tmpFile.Write(data)
	tmpFile.Close()

	return tmpFile.Name()
}

func testUploadLimit(testFile string, sumReqs int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var startWg sync.WaitGroup

	startWg.Add(sumReqs)

	successCount := 0
	rateLimitedCount := 0
	otherErrorCount := 0

	startTime := time.Now()

	for i := 1; i <= sumReqs; i++ {
		wg.Add(1)

		go func(id int){
			defer wg.Done()

			startWg.Done()
			startWg.Wait()

			client, conn, err := newConn()
			if err != nil {
				fmt.Printf("Failed to create connection %d: %v\n", id, err)
				return
			}
			defer conn.Close()

			err = uploadFileQuiet(client, testFile)

			mu.Lock()
			if err == nil {
				successCount++
				fmt.Printf("Request %2d: SUCCESS\n", id)
			} else if status.Code(err) == codes.ResourceExhausted {
				rateLimitedCount++
				fmt.Printf("Request %2d: RATE LIMITED\n", id)
			} else {
				otherErrorCount++
				fmt.Printf("Request %2d: ERROR (%v)\n", id, err)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Printf("Results (completed in %v):\n", duration)
	fmt.Printf("Successful: %d (expected ~10)\n", successCount)
	fmt.Printf("Rate limited: %d (expected ~%d)\n", rateLimitedCount, sumReqs - 10)
	fmt.Printf("Other errors: %d (expected 0)\n", otherErrorCount)

	if successCount >= 9 && successCount <= 11 {
		fmt.Println("Upload rate limiting works correctly!")
	} else {
		fmt.Println("Unexpected results. Expected ~10 successful uploads.")
	}
}

func testListLimit(sumReqs int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var wgStart sync.WaitGroup
	
	wgStart.Add(sumReqs)
	successCount := 0
	rateLimitedCount := 0

	startTime := time.Now()

	for i := 1; i <= sumReqs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
			defer cancel()

			client, conn, err := newConn()
			if err != nil {
				fmt.Printf("Failed to create connection %d: %v\n", id, err)
				return
			}
			defer conn.Close()

			wgStart.Done()
			wgStart.Wait()

			_, err = client.List(ctx, &pb.ListRequest{})

			mu.Lock()
			if err == nil {
				successCount++
			} else if status.Code(err) == codes.ResourceExhausted {
				rateLimitedCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Printf("Results (completed in %v):\n", duration)
	fmt.Printf("Successful: %d (expected ~100)\n", successCount)
	fmt.Printf("Rate limited: %d (expected ~%d)\n", rateLimitedCount, sumReqs-100)

	if successCount >= 95 && successCount <= 105 {
		fmt.Println("List rate limiting works correctly!")
	} else {
		fmt.Println("Unexpected results. Expected ~100 successful requests.")
	}
}

func uploadFileQuiet(client pb.FileServiceClient, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.Upload(ctx)
	if err != nil {
		return err
	}

	err = stream.Send(&pb.UploadRequest{
		Data: &pb.UploadRequest_Filename{
			Filename: filepath.Base(filePath),
		},
	})
	if err != nil {
		_, recvErr := stream.CloseAndRecv()
		if recvErr != nil {
			return recvErr
		}
		return err
	}

	buffer := make([]byte, chunkSize)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = stream.Send(&pb.UploadRequest{
			Data: &pb.UploadRequest_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			_, recvErr := stream.CloseAndRecv()
			if recvErr != nil {
				return recvErr
			}
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	return err
}

func newConn() (pb.FileServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	client := pb.NewFileServiceClient(conn)
	return client, conn, nil
}

func uploadFile(client pb.FileServiceClient, path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Printf("Failed to get file info: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
	defer cancel()

	stream, err := client.Upload(ctx)
	if err != nil {
		handleError(err, "upload")
		return
	}

	err = stream.Send(&pb.UploadRequest{
		Data: &pb.UploadRequest_Filename{
			Filename: filepath.Base(path),
		},
	})
	if err != nil {
		_, recvErr := stream.CloseAndRecv()
		if recvErr != nil {
			handleError(recvErr, "upload")
			return
		}
		fmt.Printf("Failed to send file name: %v\n", err)
		return
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
			fmt.Printf("Failed to read file: %v\n", err)
			return
		}

		err = stream.Send(&pb.UploadRequest{
			Data: &pb.UploadRequest_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			_, recvErr := stream.CloseAndRecv()
			if recvErr != nil {
				handleError(recvErr, "upload")
				return
			}
			fmt.Printf("Failed to send chunk: %v\n", err)
			return
		}

		totalSent += n
		progress := float64(totalSent) / float64(fileInfo.Size()) * 100
		fmt.Printf("\rProgress: %.1f%%", progress)
	}

	fmt.Println()

	response, err := stream.CloseAndRecv()
	if err != nil {
		handleError(err, "upload")
		return
	}

	fmt.Printf("Upload successful!\n")
	fmt.Printf("File ID: %s\n", response.Id)
}

func downloadFile(client pb.FileServiceClient, fileID string, outputPath string) {
	var file *os.File

	err := os.MkdirAll(outputPath, 0755)
	if err != nil {
		fmt.Printf("Failed to create directory: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
	defer cancel()
	
	stream, err := client.Download(ctx, &pb.DownloadRequest{Id: fileID})
	if err != nil {
		handleError(err, "download")
		return
	}

	for {
		resp, err := stream.Recv()
		
		if err == io.EOF {
			break
		}

		if err != nil {
			if file != nil {
				file.Close()
				os.Remove(file.Name())
			}
			handleError(err, "download")
			return
		}

		if file == nil {
			file, err = os.Create(filepath.Join(outputPath, resp.GetInfo().GetName()))
			if err != nil {
				fmt.Printf("Failed to create file: %v\n", err)
				return
			}
			defer file.Close()
		}

		file.Write(resp.GetChunk())
	}

	fmt.Println("Download successful!")
}

func listFile(client pb.FileServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
	defer cancel()

	resp, err := client.List(ctx, &pb.ListRequest{})
	if err != nil {
		handleError(err, "list")
		return
	}

	items := resp.GetItems()

	if len(items) == 0 {
		fmt.Println("No files found.")
		return
	}

	fmt.Println("List Files:")

	for _, item := range items {
		fmt.Printf(
			"ID: %v | FileName: %v | Created_At: %v | Updated_At: %v\n",
			item.Id,
			item.Name,
			item.CreatedAt.AsTime(),
			item.UpdatedAt.AsTime(),
		)
	}
}

func handleError(err error, operation string) {
	st, ok := status.FromError(err)
	
	if !ok {
		fmt.Printf("Error: %s failed: %v\n", operation, err)
		return
	}

	switch st.Code() {
	case codes.ResourceExhausted:
		fmt.Printf("Rate limit exceeded: Too many concurrent %s requests.\n", operation)
		fmt.Println("Please try again in a few seconds.")
		
	case codes.NotFound:
		fmt.Printf("File not found: %s\n", st.Message())
		
	case codes.InvalidArgument:
		fmt.Printf("Invalid request: %s\n", st.Message())
		
	case codes.Internal:
		fmt.Printf("Server error: %s\n", st.Message())
		
	case codes.DeadlineExceeded:
		fmt.Printf("Request timeout: Operation took too long.\n")
		
	case codes.Unavailable:
		fmt.Printf("Server unavailable: Cannot connect to server.\n")
		
	default:
		fmt.Printf("Error: %s failed: %s (code: %s)\n", operation, st.Message(), st.Code())
	}
}