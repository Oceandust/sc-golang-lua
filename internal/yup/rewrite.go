package yup

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type RewriteResult struct {
	UpdatedEntries int
	OutputPath     string
}

func RewriteManifest(manifestPath string, rootDir string, outputPath string) (RewriteResult, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return RewriteResult{}, err
	}

	document, err := Parse(raw)
	if err != nil {
		return RewriteResult{}, err
	}

	infoValue, err := manifestInfo(document)
	if err != nil {
		return RewriteResult{}, err
	}

	fileEntries, err := manifestFileEntries(document)
	if err != nil {
		return RewriteResult{}, err
	}

	piecesWriter, err := newPiecesWriter(infoValue)
	if err != nil {
		return RewriteResult{}, err
	}

	updated := 0
	for _, entry := range fileEntries {
		if isPadEntry(entry) {
			if piecesWriter == nil {
				continue
			}

			padLength := piecesWriter.RequiredPadding()
			entry.Set("length", IntValue(padLength))
			if err := piecesWriter.WriteZeroes(padLength); err != nil {
				return RewriteResult{}, err
			}
			continue
		}

		relativeSegments, ok := manifestPathSegments(entry)
		if !ok || len(relativeSegments) == 0 {
			continue
		}

		candidatePath := filepath.Join(append([]string{rootDir}, relativeSegments...)...)
		info, statErr := os.Stat(candidatePath)
		if statErr != nil || info.IsDir() {
			if piecesWriter != nil {
				return RewriteResult{}, fmt.Errorf("manifest piece hashing requires %s to exist under %s", stringsJoin(relativeSegments, "/"), rootDir)
			}
			continue
		}

		hash, err := fileSHA1(candidatePath)
		if err != nil {
			return RewriteResult{}, err
		}

		entry.Set("length", IntValue(info.Size()))
		entry.Set("mtime", IntValue(info.ModTime().Unix()))
		entry.Set("sha1", BytesValue(hash))
		if piecesWriter != nil {
			if err := piecesWriter.WriteFile(candidatePath); err != nil {
				return RewriteResult{}, err
			}
		}
		updated++
	}

	if piecesWriter != nil {
		infoValue.Set("pieces", BytesValue(piecesWriter.Finish()))
	}

	if outputPath == "" {
		outputPath = manifestPath
	}

	encoded, err := Encode(document)
	if err != nil {
		return RewriteResult{}, err
	}
	if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
		return RewriteResult{}, err
	}

	return RewriteResult{
		UpdatedEntries: updated,
		OutputPath:     outputPath,
	}, nil
}

func manifestInfo(document *Value) (*Value, error) {
	infoValue, ok := document.Get("info")
	if !ok {
		return nil, fmt.Errorf("manifest is missing top-level info dictionary")
	}
	return infoValue, nil
}

func manifestFileEntries(document *Value) ([]*Value, error) {
	infoValue, err := manifestInfo(document)
	if err != nil {
		return nil, err
	}

	filesValue, ok := infoValue.Get("files")
	if !ok {
		return nil, fmt.Errorf("manifest is missing info/files list")
	}

	files, ok := filesValue.AsList()
	if !ok {
		return nil, fmt.Errorf("manifest info/files is not a list")
	}

	return files, nil
}

func manifestPathSegments(entry *Value) ([]string, bool) {
	pathValue, ok := entry.Get("path")
	if !ok {
		return nil, false
	}

	items, ok := pathValue.AsList()
	if !ok {
		return nil, false
	}

	segments := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.AsString()
		if !ok {
			return nil, false
		}
		segments = append(segments, text)
	}

	return segments, true
}

func fileSHA1(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func isPadEntry(entry *Value) bool {
	attrValue, ok := entry.Get("attr")
	if !ok {
		return false
	}

	attrText, ok := attrValue.AsString()
	return ok && bytes.Contains([]byte(attrText), []byte("p"))
}

type piecesWriter struct {
	pieceLength int64
	pieceData   []byte
	pieces      bytes.Buffer
}

func newPiecesWriter(infoValue *Value) (*piecesWriter, error) {
	piecesValue, hasPieces := infoValue.Get("pieces")
	pieceLengthValue, hasPieceLength := infoValue.Get("piece length")
	if !hasPieces && !hasPieceLength {
		return nil, nil
	}
	if !hasPieces || !hasPieceLength {
		return nil, fmt.Errorf("manifest must contain both info/pieces and info/piece length")
	}

	if _, ok := piecesValue.AsBytes(); !ok {
		return nil, fmt.Errorf("manifest info/pieces is not a byte string")
	}

	pieceLength, ok := pieceLengthValue.AsInt()
	if !ok || pieceLength <= 0 {
		return nil, fmt.Errorf("manifest info/piece length is invalid")
	}

	return &piecesWriter{
		pieceLength: pieceLength,
		pieceData:   make([]byte, 0, pieceLength),
	}, nil
}

func (writer *piecesWriter) WriteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	buffer := make([]byte, 64*1024)
	for {
		count, readErr := file.Read(buffer)
		if count > 0 {
			if err := writer.write(buffer[:count]); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func (writer *piecesWriter) WriteZeroes(length int64) error {
	if length <= 0 {
		return nil
	}

	zeroes := make([]byte, minInt64(length, 64*1024))
	remaining := length
	for remaining > 0 {
		chunkLength := minInt64(remaining, int64(len(zeroes)))
		if err := writer.write(zeroes[:chunkLength]); err != nil {
			return err
		}
		remaining -= chunkLength
	}
	return nil
}

func (writer *piecesWriter) RequiredPadding() int64 {
	if writer == nil || writer.pieceLength <= 0 {
		return 0
	}
	remainder := int64(len(writer.pieceData)) % writer.pieceLength
	if remainder == 0 {
		return 0
	}
	return writer.pieceLength - remainder
}

func (writer *piecesWriter) Finish() []byte {
	if writer == nil {
		return nil
	}
	if len(writer.pieceData) > 0 {
		sum := sha1.Sum(writer.pieceData)
		_, _ = writer.pieces.Write(sum[:])
		writer.pieceData = writer.pieceData[:0]
	}
	return append([]byte(nil), writer.pieces.Bytes()...)
}

func (writer *piecesWriter) write(data []byte) error {
	for len(data) > 0 {
		remaining := int(writer.pieceLength) - len(writer.pieceData)
		if remaining <= 0 {
			sum := sha1.Sum(writer.pieceData)
			if _, err := writer.pieces.Write(sum[:]); err != nil {
				return err
			}
			writer.pieceData = writer.pieceData[:0]
			remaining = int(writer.pieceLength)
		}

		chunkLength := remaining
		if len(data) < chunkLength {
			chunkLength = len(data)
		}
		writer.pieceData = append(writer.pieceData, data[:chunkLength]...)
		data = data[chunkLength:]

		if len(writer.pieceData) == int(writer.pieceLength) {
			sum := sha1.Sum(writer.pieceData)
			if _, err := writer.pieces.Write(sum[:]); err != nil {
				return err
			}
			writer.pieceData = writer.pieceData[:0]
		}
	}
	return nil
}

func minInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func stringsJoin(items []string, separator string) string {
	if len(items) == 0 {
		return ""
	}

	length := 0
	for _, item := range items {
		length += len(item)
	}
	length += len(separator) * (len(items) - 1)

	buffer := bytes.NewBuffer(make([]byte, 0, length))
	for index, item := range items {
		if index > 0 {
			_, _ = buffer.WriteString(separator)
		}
		_, _ = buffer.WriteString(item)
	}
	return buffer.String()
}
