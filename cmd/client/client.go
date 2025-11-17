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
	
	case "test-limits":
		testRateLimits(client)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func testRateLimits(client pb.FileServiceClient) {
	fmt.Println("Testing rate limits...")

	testFile := createTestFile()
	defer os.Remove(testFile)

	fmt.Println("Test 1: Upload rate limit (max 10 concurrent)")
	fmt.Println("Sending 15 concurrent upload requests...")

	testUploadLimit(testFile, 15)

	time.Sleep(time.Second * 2)

	fmt.Println("Test 2: List rate limit (max 100 concurrent)")
	fmt.Println("Sending 105 concurrent list requests...")

	testListLimit(client, 105)

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

			err := uploadFileQuietNewConn(testFile)

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

func testListLimit(client pb.FileServiceClient, sumReqs int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	successCount := 0
	rateLimitedCount := 0

	startTime := time.Now()

	for i := 1; i <= sumReqs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := client.List(ctx, &pb.ListRequest{})

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
	fmt.Printf("Successful:    %d (expected: ~100)\n", successCount)
	fmt.Printf("Rate Limited:  %d (expected: ~%d)\n", rateLimitedCount, sumReqs-100)

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
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	if err == io.EOF {
		return nil
	}

	return nil
}

func uploadFileQuietNewConn(filePath string) error {
	conn, err := grpc.NewClient(
        serverAddr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        return err
    }
    defer conn.Close()

    client := pb.NewFileServiceClient(conn)
    return uploadFileQuiet(client, filePath)
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