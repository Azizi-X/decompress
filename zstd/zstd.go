package zstd

/*
#include <stdlib.h>
#include <stdarg.h>
#include <stdio.h>
#include <string.h>
#define ZSTD_STATIC_LINKING_ONLY
#include "zstd.h"

typedef struct {
    ZSTD_DCtx* dctx;
    ZSTD_outBuffer out;
	size_t pos;

	const char* err;
} ZstdDCtxWithBuffer;

static int outCap = 8 * 1024;
static int debug = 0;
static int shrink = 0;

void zstd_set_debug(int enable) {
    debug = enable;
}

void zstd_set_shrink(int enable) {
	shrink = enable;
}

void zstd_set_cap(int cap) {
	outCap = cap;
}

void zstd_debug_printf(const char* fmt, ...) {
    if (!debug) return;

    va_list args;
    va_start(args, fmt);
    vprintf(fmt, args);
    va_end(args);
}

static ZstdDCtxWithBuffer* zstd_create_ctx() {
    ZstdDCtxWithBuffer* ctx = malloc(sizeof(*ctx));
    if (!ctx) {
        zstd_debug_printf("[zstd_create_ctx] Failed to allocate context struct\n");
        return NULL;
    }

    ctx->dctx = ZSTD_createDCtx();
    if (!ctx->dctx) {
        zstd_debug_printf("[zstd_create_ctx] Failed to create ZSTD_DCtx\n");
        free(ctx);
        return NULL;
    }

	ZSTD_DCtx_setParameter(ctx->dctx, ZSTD_d_forceIgnoreChecksum, 1);
	ZSTD_DCtx_setParameter(ctx->dctx, ZSTD_d_format, ZSTD_f_zstd1);

    ctx->out.dst = malloc(outCap);
    ctx->out.size = outCap;
    ctx->out.pos = 0;
	ctx->err = NULL;

    zstd_debug_printf("[zstd_create_ctx] Context created with buffer capacity: %zu\n", outCap);
    return ctx;
}

static void zstd_free_ctx(ZstdDCtxWithBuffer* ctx) {
    if (!ctx) return;
    zstd_debug_printf("[zstd_free_ctx] Freeing context and buffer\n");
    ZSTD_freeDCtx(ctx->dctx);
    free(ctx->out.dst);
    free(ctx);
}

static int zstd_resize_outbuf(ZstdDCtxWithBuffer* ctx, size_t newCap, int copy)
{
    void* newBuf;

    if (copy) {
        newBuf = realloc(ctx->out.dst, newCap);
    } else {
        newBuf = malloc(newCap);
		if (newBuf) {
			free(ctx->out.dst);
		}
    }

	if (!newBuf) {
		zstd_debug_printf("[zstd_resize_outbuf] Failed to reallocate to %zu bytes\n", newCap);
		return 0;
	}

    zstd_debug_printf(
        "[zstd_resize_outbuf] Resized output buffer to %zu (old: %zu, copy: %d)\n",
        newCap, ctx->out.size, copy
    );

	ctx->out.dst = newBuf;
    ctx->out.size = newCap;

    return 1;
}

static void zstd_stream_decompress(ZstdDCtxWithBuffer* ctx,
    const void* src, size_t srcSize)
{
	if (shrink && ctx->out.size > outCap) {
		zstd_resize_outbuf(ctx, outCap, 0);
	}

	ctx->out.pos = 0;

	size_t prev_in_pos = 0;
	size_t prev_out_pos = 0;

    ZSTD_inBuffer in = { src, srcSize, 0 };

	while (1) {
		ctx->out.pos = prev_out_pos;
		prev_in_pos = in.pos;

		size_t ret = ZSTD_decompressStream(ctx->dctx, &ctx->out, &in);

		if (ctx->out.pos > prev_out_pos) {
			ctx->pos = ctx->out.pos;
		}

		int made_forward_progress = in.pos > prev_in_pos || ctx->out.pos > prev_out_pos;
		int fully_processed_input = in.pos == in.size;

		if (ret == 0 || (!made_forward_progress && fully_processed_input)) {
			zstd_debug_printf("[zstd_stream_decompress] Decompressed %zu/%zu bytes, input offset: %zu/%zu\n",
				ctx->pos, ctx->out.size, in.pos, srcSize);
			return;
		} else if (ret > 0) {
			if (!made_forward_progress && !fully_processed_input) {
				ctx->err = "corrupted data";
				return;
			}
		} else {
			ctx->err = "bad arg";
			return;
		}

		if (ctx->out.pos == ctx->out.size && made_forward_progress) {
			prev_out_pos = ctx->out.pos;
			size_t newCap = ctx->out.size * 2;
			if (zstd_resize_outbuf(ctx, newCap, 1) == 0) {
				ctx->err = "failed to resize output buffer";
				return;
			}
		}
	}
}
*/
import "C"
import (
	"errors"
	"sync"
	"unsafe"
)

var (
	ErrContextClosed  = errors.New("zstd: context is closed")
	ErrEmptyData      = errors.New("zstd: empty data")
	ErrFailedToCreate = errors.New("zstd: failed to create context")
)

type zstd struct {
	ctx *C.ZstdDCtxWithBuffer
	mu  sync.Mutex
}

func New() (*zstd, error) {
	c := C.zstd_create_ctx()
	if c == nil {
		return nil, ErrFailedToCreate
	}

	decompressor := &zstd{
		ctx: c,
	}

	return decompressor, nil
}

func (z *zstd) Close() {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.ctx != nil {
		C.zstd_free_ctx(z.ctx)
		z.ctx = nil
	}
}

func SetDebug(enabled bool) {
	C.zstd_set_debug(boolToCint(enabled))
}

func SetShrink(enabled bool) {
	C.zstd_set_shrink(boolToCint(enabled))
}

func boolToCint(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

func (z *zstd) Decompress(data []byte) ([]byte, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.ctx == nil {
		return nil, ErrContextClosed
	} else if len(data) == 0 {
		return nil, ErrEmptyData
	}

	C.zstd_stream_decompress(
		z.ctx,
		unsafe.Pointer(&data[0]),
		C.size_t(len(data)),
	)

	if z.ctx.err != nil {
		return nil, errors.New(C.GoString(z.ctx.err))
	}

	return unsafe.Slice((*byte)(z.ctx.out.dst), z.ctx.pos), nil
}
