package codex

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"time"
)

func streamLogFile(ctx context.Context, path string, follow bool, lines int, w io.Writer) error {
	file, err := openLogFile(ctx, path, follow)
	if err != nil {
		return err
	}
	defer file.Close()

	offset, err := writeLastLines(file, lines, w)
	if err != nil || !follow {
		return err
	}
	return followLogFile(ctx, file, offset, w)
}

func openLogFile(ctx context.Context, path string, follow bool) (*os.File, error) {
	for {
		file, err := os.Open(path)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrNotExist) || !follow {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(logPollInterval):
		}
	}
}

func writeLastLines(file *os.File, lines int, w io.Writer) (int64, error) {
	if lines <= 0 {
		lines = 20
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	size := info.Size()
	if size == 0 {
		return 0, nil
	}
	data := make([]byte, size)
	if _, err := file.ReadAt(data, 0); err != nil && err != io.EOF {
		return 0, err
	}
	parts := bytes.Split(data, []byte{'\n'})
	if len(parts) > lines+1 {
		parts = parts[len(parts)-lines-1:]
	}
	out := bytes.Join(parts, []byte{'\n'})
	if len(out) > 0 {
		if _, err := w.Write(out); err != nil {
			return 0, err
		}
	}
	return size, nil
}

func followLogFile(ctx context.Context, file *os.File, offset int64, w io.Writer) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(logPollInterval):
		}

		info, err := file.Stat()
		if err != nil {
			return err
		}
		if info.Size() <= offset {
			continue
		}
		chunk := make([]byte, info.Size()-offset)
		n, err := file.ReadAt(chunk, offset)
		if err != nil && err != io.EOF {
			return err
		}
		if n > 0 {
			if _, err := w.Write(chunk[:n]); err != nil {
				return err
			}
			offset += int64(n)
		}
	}
}
