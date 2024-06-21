package service

import (
	"fmt"
	"mime/multipart"
	"net/http"
)

func (service *Service) ValidateImageMimeType(header *multipart.FileHeader) error {
	file, err := header.Open()
	if err != nil {
		return err
	}

	fileHeader := make([]byte, 512)
	if _, err = file.Read(fileHeader); err != nil {
		return err
	}

	mimeType := http.DetectContentType(fileHeader)

	switch mimeType {
	case "image/jpeg":
	case "image/jpg":
	case "image/png":
	case "image/gif":
	default:
		return fmt.Errorf("image type %s not allowed", mimeType)
	}

	return nil
}
