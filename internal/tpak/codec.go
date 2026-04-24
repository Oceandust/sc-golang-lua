package tpak

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

func rawDeflate(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := flate.NewWriter(&buffer, flate.BestCompression)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func rawInflate(data []byte, expectedSize int) ([]byte, error) {
	var buffer bytes.Buffer
	if expectedSize > 0 {
		buffer.Grow(expectedSize)
	}

	if err := rawInflateToWriter(bytes.NewReader(data), &buffer); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func rawInflateToWriter(source io.Reader, destination io.Writer) error {
	reader := flate.NewReader(source)
	defer func() {
		_ = reader.Close()
	}()

	_, err := io.Copy(destination, reader)
	return err
}

func xorFirstWord(data []byte, mask uint32) error {
	if len(data) < 4 {
		return fmt.Errorf("compressed block is too small to xor first word")
	}

	word := binary.LittleEndian.Uint32(data[:4])
	binary.LittleEndian.PutUint32(data[:4], word^mask)
	return nil
}

func alignOffset(offset int64) int64 {
	remainder := offset % tableAlignment
	if remainder == 0 {
		return offset
	}
	return offset + (tableAlignment - remainder)
}

func alignmentPadding(offset int64) []byte {
	aligned := alignOffset(offset)
	if aligned == offset {
		return nil
	}
	return make([]byte, aligned-offset)
}

func archivePathFromRelative(relativePath string) string {
	return strings.ReplaceAll(relativePath, "/", `\`)
}

func relativePathFromArchive(archivePath string) string {
	return strings.ReplaceAll(archivePath, `\`, "/")
}
