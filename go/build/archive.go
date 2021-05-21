package build

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func createZipArchive(dst string, files []string) error {
	zipFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create zip archive: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open binary file: %w", err)
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat binary file: %w", err)
		}

		header, err := zip.FileInfoHeader(stat)
		if err != nil {
			return fmt.Errorf("failed to create file info header: %w", err)
		}

		subFile, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip header: %w", err)
		}
		header.Name = filepath.Base(filePath)

		_, err = io.Copy(subFile, file)
		if err != nil {
			return fmt.Errorf("failed to write binary to zip archive: %w", err)
		}
	}

	return nil
}

func createTarGzArchive(dst string, files []string) error {
	tarFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	defer tarFile.Close()

	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open binary file: %w", err)
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat binary file: %w", err)
		}

		header, err := tar.FileInfoHeader(stat, "")
		if err != nil {
			return fmt.Errorf("failed to stat binary file: %w", err)
		}
		header.Name = filepath.Base(filePath)

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		_, err = io.Copy(tarWriter, file)
		if err != nil {
			return fmt.Errorf("failed to write binary to tar archive: %w", err)
		}
	}

	return nil
}
