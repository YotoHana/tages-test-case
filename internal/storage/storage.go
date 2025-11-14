package storage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
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

func (s *Storage) GetFileList() {

}

func (s *Storage) GetFile() {

}