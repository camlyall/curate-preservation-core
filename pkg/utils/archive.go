package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
)

// ----------------------------
// Helper Functions
// ----------------------------

// validatePath ensures that target is within destDir (prevents ZipSlip).
func validatePath(target, destDir string) error {
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(target), cleanDest) {
		return fmt.Errorf("illegal file path: %s", target)
	}
	return nil
}

// ----------------------------
// Detection Functions
// ----------------------------

// IsZipFile checks if a file is a ZIP archive by reading its signature.
func IsZipFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	var signature [4]byte
	if _, err = file.Read(signature[:]); err != nil {
		return false
	}
	// ZIP file signature: 0x50 0x4B 0x03 0x04
	return signature == [4]byte{0x50, 0x4B, 0x03, 0x04}
}

// Is7zFile checks if a file is a 7-Zip archive by comparing its header signature.
func Is7zFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	var header [6]byte
	if _, err = file.Read(header[:]); err != nil {
		return false
	}
	expected := []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}
	return bytes.Equal(header[:], expected)
}

// IsTarFile attempts to detect a tar archive by checking for the "ustar" magic.
// (Tar files don’t always have a unique signature; this checks for POSIX tar.)
func IsTarFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// POSIX tar header has magic "ustar" at offset 257.
	if _, err := file.Seek(257, io.SeekStart); err != nil {
		return false
	}
	buf := make([]byte, 6)
	n, err := file.Read(buf)
	if err != nil || n < 6 {
		return false
	}
	return strings.HasPrefix(string(buf), "ustar")
}

// ----------------------------
// Extraction Functions
// ----------------------------

// ExtractZip extracts the ZIP archive at src into dest.
// It validates file paths (ZipSlip check), uses os.Mkdir for directories,
// and returns the computed package name (dest/packageName).
func ExtractZip(ctx context.Context, src, dest string) (string, error) {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return "", fmt.Errorf("failed to open zip file %q: %w", src, err)
	}
	defer reader.Close()

	// Ensure destination exists.
	if err := CreateDir(dest); err != nil {
		return "", fmt.Errorf("failed to create destination directory %q: %w", dest, err)
	}
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)

	for _, file := range reader.File {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		filePath := filepath.Join(cleanDest, file.Name)
		if err := validatePath(filePath, cleanDest); err != nil {
			return "", fmt.Errorf("invalid file path %q: %w", filePath, err)
		}
		if file.FileInfo().IsDir() {
			if err := CreateDir(filePath); err != nil {
				return "", fmt.Errorf("failed to create directory %q: %w", filePath, err)
			}
			continue
		}

		if err := CreateDir(filepath.Dir(filePath)); err != nil {
			return "", fmt.Errorf("failed to create parent directories for %q: %w", filePath, err)
		}

		outFile, err := os.Create(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to create file %q: %w", filePath, err)
		}
		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return "", fmt.Errorf("failed to open file %q in archive: %w", file.Name, err)
		}
		if _, err := io.Copy(outFile, rc); err != nil {
			rc.Close()
			outFile.Close()
			return "", fmt.Errorf("failed to copy contents to %q: %w", filePath, err)
		}
		rc.Close()
		outFile.Close()
	}

	packageName := filepath.Base(strings.TrimSuffix(src, filepath.Ext(src)))
	extractedPath := filepath.Join(cleanDest, packageName)
	return extractedPath, nil
}

// Extract7z extracts the 7z archive at src into dest using similar logic.
func Extract7z(ctx context.Context, src, dest string) (string, error) {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return "", fmt.Errorf("opening archive: %w", err)
	}
	defer r.Close()

	// Ensure destination exists. Parents must exist.
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		if err := os.Mkdir(dest, 0755); err != nil {
			return "", fmt.Errorf("creating destination directory: %w", err)
		}
	}
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)

	for _, file := range r.File {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		outPath := filepath.Join(cleanDest, file.Name)
		if err := validatePath(outPath, cleanDest); err != nil {
			return "", err
		}
		if file.FileHeader.FileInfo().IsDir() {
			if err := os.Mkdir(outPath, file.Mode()); err != nil && !os.IsExist(err) {
				return "", fmt.Errorf("creating directory %q: %w", outPath, err)
			}
			continue
		}

		parentDir := filepath.Dir(outPath)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			if err := os.Mkdir(parentDir, 0755); err != nil {
				return "", fmt.Errorf("creating parent directories for %q: %w", outPath, err)
			}
		}

		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("opening file %q from archive: %w", file.Name, err)
		}
		outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("creating file %q: %w", outPath, err)
		}
		if _, err := io.Copy(outFile, rc); err != nil {
			rc.Close()
			outFile.Close()
			return "", fmt.Errorf("copying contents to %q: %w", outPath, err)
		}
		rc.Close()
		outFile.Close()
	}

	packageName := filepath.Base(strings.TrimSuffix(src, filepath.Ext(src)))
	extractedPath := filepath.Join(cleanDest, packageName)
	return extractedPath, nil
}

// ExtractTar extracts a TAR or TAR.GZ archive at src into dest.
// It performs a ZipSlip-like check and returns the computed package name.
func ExtractTar(ctx context.Context, src, dest string) (string, error) {
	file, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var tarReader *tar.Reader
	if strings.HasSuffix(src, ".gz") || strings.HasSuffix(src, ".tgz") {
		gr, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer gr.Close()
		tarReader = tar.NewReader(gr)
	} else {
		tarReader = tar.NewReader(file)
	}

	// Ensure destination exists. Parents must exist.
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		if err := os.Mkdir(dest, 0755); err != nil {
			return "", err
		}
	}
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		header, err := tarReader.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return "", err
		}
		filePath := filepath.Join(cleanDest, header.Name)
		if err := validatePath(filePath, cleanDest); err != nil {
			return "", err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(filePath, os.FileMode(header.Mode)); err != nil && !os.IsExist(err) {
				return "", err
			}
		case tar.TypeReg:
			parentDir := filepath.Dir(filePath)
			if _, err := os.Stat(parentDir); os.IsNotExist(err) {
				if err := os.Mkdir(parentDir, 0755); err != nil {
					return "", err
				}
			}
			outFile, err := os.Create(filePath)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()
		}
	}

	packageName := filepath.Base(strings.TrimSuffix(src, filepath.Ext(src)))
	extractedPath := filepath.Join(cleanDest, packageName)
	return extractedPath, nil
}

func ExtractArchive(ctx context.Context, src, dest string) (string, error) {
	var aipPath string
	var err error
	if Is7zFile(src) {
		aipPath, err = Extract7z(ctx, src, dest)
		if err != nil {
			return "", fmt.Errorf("error extracting 7zip: %w", err)
		}
	} else if IsTarFile(src) {
		aipPath, err = ExtractTar(ctx, src, dest)
		if err != nil {
			return "", fmt.Errorf("error extracting tar: %w", err)
		}
	} else if IsZipFile(src) {
		aipPath, err = ExtractZip(ctx, src, dest)
		if err != nil {
			return "", fmt.Errorf("error extracting zip: %w", err)
		}
	} else {
		return "", fmt.Errorf("archive is not in a supported format: %s", src)
	}
	if aipPath == "" {
		return "", fmt.Errorf("error extracting archive: %s", src)
	}
	return aipPath, nil
}

// ----------------------------
// Compression Functions
// ----------------------------

// CompressToZip compresses the contents of the src directory into a ZIP archive at dest.
func CompressToZip(ctx context.Context, src, dest string) error {
	zipFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return fmt.Errorf("walking path: %w", err)
		}
		// Compute relative path.
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}
		// Skip the root directory.
		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("creating zip header: %w", err)
		}
		header.Name = relPath
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writerEntry, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("creating zip entry: %w", err)
		}
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("opening file: %w", err)
			}
			defer file.Close()

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if _, err := io.Copy(writerEntry, file); err != nil {
				return fmt.Errorf("copying file contents: %w", err)
			}
		}
		return nil
	})
}
