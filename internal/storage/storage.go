package storage

import (
	"os"
	"path/filepath"
	"strings"

	pb "github.com/YotoHana/tages-test-case/api/proto"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	storageRoot = "./uploads"
)

type Storage struct{}

func New() (*Storage, error) {
	if err := os.MkdirAll(storageRoot, 0755); err != nil {
		return nil, err
	}
	return &Storage{}, nil
}

func (s *Storage) CreateFile(fileName string) (file *os.File, id string, err error) {
	id = uuid.NewString()
	resultName := strings.Join([]string{id, fileName}, "_")
	file, err = os.Create(filepath.Join(storageRoot, resultName))
	if err != nil {
		return nil, "", err
	}

	return file, id, nil
}

func (s *Storage) GetFileList() (items []*pb.ListResponse_Item, err error) {
	entry, err := os.ReadDir(storageRoot)
	if err != nil {
		return nil, err
	}

	items = make([]*pb.ListResponse_Item, 0, len(entry))

	for _, e := range entry {
		fileInfo, err := e.Info()
		if err != nil {
			return nil, err
		}

		fileName := strings.Split(e.Name(), "_")

		ts := timestamppb.New(fileInfo.ModTime())

		item := &pb.ListResponse_Item{
			Id: fileName[0],
			Name: fileName[1],
			CreatedAt: ts,
			UpdatedAt: ts,
		}

		items = append(items, item)
	}

	return items, nil
}

func (s *Storage) GetFile() {

}