package zstd

/*
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#define ZSTD_STATIC_LINKING_ONLY
#include "zstd.h"

typedef struct {
    ZSTD_DCtx*     dctx;
    void*          outBuf;
    size_t         outCap;
    ZSTD_outBuffer out;
} ZstdDCtxWithBuffer;

static int debug = 0;
static int shrink = 0;

void set_debug(int enable) {
    debug = enable;
}

void set_shrink(int enable) {
	shrink = enable;
}

void debug_printf(const char* fmt, ...) {
    if (!debug) return;

    va_list args;
    va_start(args, fmt);
    vprintf(fmt, args);
    va_end(args);
}

static ZstdDCtxWithBuffer* zstd_create_ctx() {
    size_t outCap = ZSTD_DStreamOutSize();
    debug_printf("[zstd_create_ctx] Initial output buffer size: %zu bytes\n", outCap);

    ZstdDCtxWithBuffer* ctx = malloc(sizeof(*ctx));
    if (!ctx) {
        debug_printf("[zstd_create_ctx] Failed to allocate context struct\n");
        return NULL;
    }

    ctx->dctx = ZSTD_createDCtx();
    if (!ctx->dctx) {
        debug_printf("[zstd_create_ctx] Failed to create ZSTD_DCtx\n");
        free(ctx);
        return NULL;
    }

	ZSTD_DCtx_setParameter(ctx->dctx, ZSTD_d_forceIgnoreChecksum, 1);
	ZSTD_DCtx_setParameter(ctx->dctx, ZSTD_d_format, ZSTD_f_zstd1);

    ctx->outBuf = malloc(outCap);
    ctx->outCap = outCap;
    ctx->out.dst = ctx->outBuf;
    ctx->out.size = outCap;
    ctx->out.pos = 0;

    debug_printf("[zstd_create_ctx] Context created with buffer capacity: %zu\n", outCap);
    return ctx;
}

static void zstd_free_ctx(ZstdDCtxWithBuffer* ctx) {
    if (!ctx) return;
    debug_printf("[zstd_free_ctx] Freeing context and buffer\n");
    ZSTD_freeDCtx(ctx->dctx);
    free(ctx->outBuf);
    free(ctx);
}

static int zstd_resize_outbuf(ZstdDCtxWithBuffer* ctx, size_t newCap) {
    size_t oldPos = ctx->out.pos;
    void* newBuf = realloc(ctx->outBuf, newCap);
    if (!newBuf) {
        debug_printf("[zstd_resize_outbuf] Failed to reallocate to %zu bytes\n", newCap);
        return 0;
    }

    ctx->outBuf = newBuf;
    ctx->outCap = newCap;
    ctx->out.dst = ctx->outBuf;
    ctx->out.size = newCap;
    ctx->out.pos = oldPos;

    debug_printf("[zstd_resize_outbuf] Resized output buffer to %zu bytes (old position: %zu)\n", newCap, oldPos);
    return 1;
}

static size_t zstd_stream_decompress(ZstdDCtxWithBuffer* ctx,
    const void* src, size_t srcSize, size_t offset,
    int* done, char** error)
{
    ZSTD_inBuffer in = { src, srcSize, offset };
    ctx->out.pos = 0;

    size_t ret = ZSTD_decompressStream(ctx->dctx, &ctx->out, &in);

	int made_forward_progress = in.pos > offset || ctx->out.pos > 0;
    int fully_processed_input = in.pos == in.size;

	if (ctx->out.pos == ctx->outCap && !fully_processed_input) {
		size_t newCap = ctx->outCap * 2;
		if (newCap < ctx->outCap+ret) {
			newCap = ctx->outCap + ret;
		}

		if (zstd_resize_outbuf(ctx, newCap) == 0) {
			debug_printf("[zstd_stream_decompress] Failed to resize output buffer\n");
			*error = "failed to resize output buffer";
			return -1;
		}

		debug_printf("[zstd_stream_decompress] Buffer resize to %zu\n", newCap);
	}

	if (ret == 0 || (!made_forward_progress && fully_processed_input)) {
		*done = 1;

	 	if (shrink) {
			size_t newCap = ZSTD_DStreamOutSize();

			if (ctx->outCap > newCap) {
				zstd_resize_outbuf(ctx, newCap);
			}
		}
	} else if (ret > 0) {
		if (!made_forward_progress && !fully_processed_input) {
			*error = "corrupted data";
			return -1;
		}
	} else {
	 	*error = "bad arg";
		return -1;
	}

    debug_printf("[zstd_stream_decompress] Decompressed %zu bytes, input offset: %zu/%zu, done: %d\n",
        ctx->out.pos, in.pos, srcSize, *done);

    return in.pos;
}
*/
import "C"
import (
	"errors"
	"sync"
	"unsafe"
)

var (
	ErrContextClosed = errors.New("zstd: context is closed")
	ErrEmptyData     = errors.New("zstd: empty data")

	MaxCap = 8 * 1024
)

type ZstdDecoder struct {
	ctx *C.ZstdDCtxWithBuffer
	buf []byte
	mu  sync.RWMutex
}

func NewDecoder() (*ZstdDecoder, error) {
	cCtx := C.zstd_create_ctx()
	if cCtx == nil {
		return nil, errors.New("zstd: failed to create context")
	}

	decoder := &ZstdDecoder{
		ctx: cCtx,
		buf: make([]byte, 0, MaxCap),
	}

	return decoder, nil
}

func (d *ZstdDecoder) Close() {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.ctx != nil {
		C.zstd_free_ctx(d.ctx)
		d.ctx = nil
	}
}

func SetDebug(enabled bool) {
	C.set_debug(boolToCInt(enabled))
}

func SetShrink(enabled bool) {
	C.set_shrink(boolToCInt(enabled))
}

func boolToCInt(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

func (d *ZstdDecoder) Decompress(data []byte) ([]byte, error) {
	d.mu.RLock()

	if d.ctx == nil {
		d.mu.RUnlock()
		return nil, ErrContextClosed
	}

	var offset int

	if cap(d.buf) > MaxCap {
		d.buf = nil
		d.buf = make([]byte, 0, MaxCap)
	} else {
		d.buf = d.buf[:0]
	}

	if cap(d.buf) < len(data) {
		d.buf = make([]byte, 0, len(data)*2)
	}

	for {
		chunk, done, err := d.streamDecompress(data, &offset)
		if err != nil {
			d.mu.RUnlock()
			return nil, err
		}
		d.buf = append(d.buf, chunk...)
		if done {
			d.mu.RUnlock()
			return d.buf, nil
		}
	}
}

func (d *ZstdDecoder) streamDecompress(data []byte, offset *int) (chunk []byte, done bool, err error) {
	if len(data) == 0 {
		return nil, false, ErrEmptyData
	}

	var cdone C.int
	var cerror *C.char

	newOff := C.zstd_stream_decompress(
		d.ctx,
		unsafe.Pointer(&data[0]),
		C.size_t(len(data)),
		C.size_t(*offset),
		&cdone,
		&cerror,
	)

	if cerror != nil {
		return nil, false, errors.New(C.GoString(cerror))
	}

	*offset = int(newOff)
	chunk = unsafe.Slice((*byte)(d.ctx.outBuf), d.ctx.out.pos)

	return chunk, cdone != 0, nil
}
