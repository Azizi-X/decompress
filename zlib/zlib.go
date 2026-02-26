package zlib

/*
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <zlib.h>

typedef struct {
    z_stream strm;
    void* outBuf;
    size_t outCap;
    size_t pos;

    int done;
    const char* err;
} ZlibDCtxWithBuffer;

static int outCap = 8 * 1024;
static int debug = 0;
static int shrink = 0;

void zlib_set_debug(int enable) {
    debug = enable;
}

void zlib_set_shrink(int enable) {
    shrink = enable;
}

void zlib_set_cap(int cap) {
	outCap = cap;
}

void zlib_debug_printf(const char* fmt, ...) {
    if (!debug) return;

    va_list args;
    va_start(args, fmt);
    vprintf(fmt, args);
    va_end(args);
}

ZlibDCtxWithBuffer* zlib_create_ctx() {
    zlib_debug_printf("[zlib_create_ctx] Initial output buffer size: %zu bytes\n", outCap);

    ZlibDCtxWithBuffer* ctx = malloc(sizeof(*ctx));
    if (!ctx) {
        zlib_debug_printf("[zlib_create_ctx] Failed to allocate context struct\n");
        return NULL;
    }

    ctx->strm.zalloc = Z_NULL;
    ctx->strm.zfree = Z_NULL;
    ctx->strm.opaque = Z_NULL;
    ctx->pos = 0;
    ctx->err = NULL;
    ctx->done = 0;

    ctx->outCap = outCap;
    ctx->outBuf = malloc(outCap);

    if (!ctx->outBuf) {
        zlib_debug_printf("[zlib_create_ctx] Failed to create ZLIB_DCtx\n");
        free(ctx);
        return NULL;
    }

    int ret = inflateInit2(&ctx->strm, -15);
    if (ret != Z_OK) {
        zlib_debug_printf("[zlib_create_ctx] Failed to create ZLIB_INFLATE\n");
        free(ctx->outBuf);
        free(ctx);
        return NULL;
    }

    return ctx;
}

void zlib_free_ctx(ZlibDCtxWithBuffer* ctx) {
    if (!ctx) return;
    zlib_debug_printf("[zlib_free_ctx] Freeing context and buffer\n");
    inflateEnd(&ctx->strm);
    free(ctx->outBuf);
    free(ctx);
}

static int zlib_resize_outbuf(ZlibDCtxWithBuffer* ctx, size_t newCap, int copy) {
    void* newBuf;

    if (copy) {
        newBuf = realloc(ctx->outBuf, newCap);
    } else {
        newBuf = malloc(newCap);
        if (newBuf) {
            free(ctx->outBuf);
        }
    }

    if (!newBuf) {
        zlib_debug_printf("[zlib_resize_outbuf] Failed to reallocate to %zu bytes\n", newCap);
        return 0;
    }

    zlib_debug_printf("[zlib_resize_outbuf] Resized output buffer to %zu (old: %zu, copy: %d)\n",
        newCap, ctx->outCap, copy);

    ctx->outBuf = newBuf;
    ctx->outCap = newCap;

    ctx->strm.next_out = (unsigned char*)ctx->outBuf + ctx->pos;
    ctx->strm.avail_out = newCap - ctx->pos;

    return 1;
}

void zlib_stream_decompress(ZlibDCtxWithBuffer* ctx,
    const void* src, size_t srcSize)
{
	if (shrink && ctx->outCap > outCap) {
        zlib_resize_outbuf(ctx, outCap, 0);
    }

    unsigned char* input = (unsigned char*)src;
    size_t remaining = srcSize;

    if (remaining >= 2 && input[0] == 0x78 && input[1] == 0xda) {
        input += 2;
        remaining -= 2;
    }

    ctx->pos = 0;
    ctx->strm.next_in = input;
    ctx->strm.avail_in = remaining;
    ctx->strm.next_out = ctx->outBuf;
    ctx->strm.avail_out = ctx->outCap;

    int ret;
    while (1) {
        ret = inflate(&ctx->strm, Z_SYNC_FLUSH);
        ctx->pos = ctx->outCap - ctx->strm.avail_out;

        if (ret != Z_OK && ret != Z_BUF_ERROR && ret != Z_STREAM_END) {
            ctx->err = "zlib inflate failed";
            return;
        }

        if (ctx->strm.avail_out == 0 && ctx->strm.avail_in > 0) {
            size_t newCap = ctx->outCap * 2;
            if (!zlib_resize_outbuf(ctx, newCap, 1)) {
                ctx->err = "failed to resize output buffer";
                return;
            }
            continue;
        }

        break;
    }

    ctx->done = (ret == Z_STREAM_END || ctx->strm.avail_in == 0);

    zlib_debug_printf(
        "[zlib_stream_decompress] Decompressed %zu/%zu bytes, done: %d\n",
        ctx->pos,
		ctx->outCap,
        ctx->done
    );
}
*/
import "C"
import (
	"errors"
	"sync"
	"unsafe"
)

var (
	ErrContextClosed  = errors.New("zlib: context is closed")
	ErrEmptyData      = errors.New("zlib: empty data")
	ErrFailedToCreate = errors.New("zlib: failed to create context")
)

type zlib struct {
	ctx *C.ZlibDCtxWithBuffer
	mu  sync.RWMutex
}

func New() (*zlib, error) {
	cCtx := C.zlib_create_ctx()
	if cCtx == nil {
		return nil, ErrFailedToCreate
	}

	decompress := &zlib{
		ctx: cCtx,
	}

	return decompress, nil
}

func (z *zlib) Close() {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.ctx != nil {
		C.zlib_free_ctx(z.ctx)
		z.ctx = nil
	}
}

func SetDebug(enabled bool) {
	C.zlib_set_debug(boolToCInt(enabled))
}

func SetShrink(enabled bool) {
	C.zlib_set_shrink(boolToCInt(enabled))
}

func SetCap(cap int) {
	C.zlib_set_cap(C.int(cap))
}

func boolToCInt(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

func (z *zlib) Decompress(data []byte) ([]byte, error) {
	z.mu.RLock()
	defer z.mu.RUnlock()

	if z.ctx == nil {
		return nil, ErrContextClosed
	} else if len(data) == 0 {
		return nil, ErrEmptyData
	}

	C.zlib_stream_decompress(
		z.ctx,
		unsafe.Pointer(&data[0]),
		C.size_t(len(data)),
	)

	if z.ctx.err != nil {
		return nil, errors.New(C.GoString(z.ctx.err))
	} else if z.ctx.done == 0 {
		panic("decompression not finished (this should never happen)")
	}

	return unsafe.Slice((*byte)(z.ctx.outBuf), z.ctx.pos), nil
}
