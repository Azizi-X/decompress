package zstd_native

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

type ZstdDecoder struct {
	*io.PipeWriter
	ctx    *zstd.Decoder
	cancel context.CancelFunc
	closed chan struct{}
	once   sync.Once
	mu     sync.RWMutex
}

func NewDecoder() (*ZstdDecoder, error) {
	return NewDecoderCtx(context.Background())
}

func NewDecoderCtx(parent context.Context) (*ZstdDecoder, error) {
	var pr, pw = io.Pipe()
	var nCtx, err = zstd.NewReader(pr)
	if err != nil {
		return nil, errors.New("zstd: failed to create context")
	}

	ctx, cancel := context.WithCancel(parent)

	decoder := &ZstdDecoder{
		PipeWriter: pw,
		ctx:        nCtx,
		cancel:     cancel,
		closed:     make(chan struct{}),
	}

	go func() {
		select {
		case <-ctx.Done():
			decoder.Close()
		case <-decoder.closed:
		}
	}()

	return decoder, nil
}

func (d *ZstdDecoder) Close() {
	d.once.Do(func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		d.cancel()
		d.PipeWriter.Close()
		d.ctx.Close()

		d.ctx = nil
		close(d.closed)
	})
}

func (d *ZstdDecoder) Decompress(data []byte) ([]byte, error) {
	go d.Write(data)

	buf := make([]byte, 4096)
	chunk := []byte{}

	for {
		n, err := d.ctx.Read(buf)

		chunk = append(chunk, buf[:n]...)

		done := n == 0 || n < len(buf)

		if done {
			return chunk, nil
		}

		if err != nil {
			return nil, err
		}
	}
}
