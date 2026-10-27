//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <Metal/Metal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "compute.h"

// Manual ownership: no NSApplication, CAMetalLayer, window or main-thread UI.
struct tf_metal {
  id<MTLDevice> device;
  id<MTLCommandQueue> queue;
  id<MTLComputePipelineState> pipeline;
  id<MTLBuffer> params;
  id<MTLBuffer> output;
  char name[256];
};

static void report(char *dst, size_t n, const char *stage, NSError *error) {
  const char *detail = error ? [[error localizedDescription] UTF8String] : NULL;
  snprintf(dst, n, "Metal %s%s%s", stage, detail ? ": " : "", detail ? detail : "");
}

tf_metal *tf_metal_open(const char *source, char *error, size_t error_size) {
  @autoreleasepool {
    tf_metal *g = calloc(1, sizeof(*g));
    if (!g) {
      report(error, error_size, "allocation failed", nil);
      return NULL;
    }
    g->device = MTLCreateSystemDefaultDevice();
    if (!g->device) {
      report(error, error_size, "device unavailable", nil);
      tf_metal_close(g);
      return NULL;
    }
    snprintf(g->name, sizeof(g->name), "%s", [[g->device name] UTF8String]);
    NSError *cause = nil;
    MTLCompileOptions *options = [[MTLCompileOptions alloc] init];
    options.languageVersion = MTLLanguageVersion2_0;
    options.fastMathEnabled = NO;
    id<MTLLibrary> library = [g->device newLibraryWithSource:[NSString stringWithUTF8String:source]
                                                   options:options error:&cause];
    [options release];
    if (!library) {
      report(error, error_size, "shader compilation failed", cause);
      tf_metal_close(g);
      return NULL;
    }
    id<MTLFunction> kernel = [library newFunctionWithName:@"main0"];
    [library release];
    if (!kernel) {
      report(error, error_size, "kernel main0 missing", nil);
      tf_metal_close(g);
      return NULL;
    }
    g->pipeline = [g->device newComputePipelineStateWithFunction:kernel error:&cause];
    [kernel release];
    if (!g->pipeline || [g->pipeline maxTotalThreadsPerThreadgroup] < 64) {
      report(error, error_size, "compute pipeline unavailable", cause);
      tf_metal_close(g);
      return NULL;
    }
    g->queue = [g->device newCommandQueue];
    // Shared buffers work on both Apple Silicon and Intel/discrete Macs.
    // waitUntilCompleted below makes GPU writes visible before copying to Go.
    g->params = [g->device newBufferWithLength:128 * sizeof(float) options:MTLResourceStorageModeShared];
    if (!g->queue || !g->params) {
      report(error, error_size, "queue/buffer allocation failed", nil);
      tf_metal_close(g);
      return NULL;
    }
    return g;
  }
}

const char *tf_metal_name(tf_metal *g) { return g->name; }

int tf_metal_run(tf_metal *g, const float *params, float *out, uint32_t pixels,
                 char *error, size_t error_size) {
  @autoreleasepool {
    if (!g || !params || !out || !pixels || pixels > (1u << 20)) {
      report(error, error_size, "invalid dispatch", nil);
      return -1;
    }
    NSUInteger bytes = (NSUInteger)pixels * 4 * sizeof(float);
    if (!g->output || [g->output length] < bytes) {
      id<MTLBuffer> next = [g->device newBufferWithLength:bytes options:MTLResourceStorageModeShared];
      if (!next) {
        report(error, error_size, "output allocation failed", nil);
        return -1;
      }
      [g->output release];
      g->output = next;
    }
    // Go pointers are only used during these synchronous copies, never retained.
    memcpy([g->params contents], params, 128 * sizeof(float));
    id<MTLCommandBuffer> command = [g->queue commandBuffer];
    id<MTLComputeCommandEncoder> encoder = [command computeCommandEncoder];
    if (!command || !encoder) {
      report(error, error_size, "command allocation failed", nil);
      return -1;
    }
    [encoder setComputePipelineState:g->pipeline];
    [encoder setBuffer:g->params offset:0 atIndex:0];
    [encoder setBuffer:g->output offset:0 atIndex:1];
    [encoder dispatchThreadgroups:MTLSizeMake((pixels + 63) / 64, 1, 1)
            threadsPerThreadgroup:MTLSizeMake(64, 1, 1)];
    [encoder endEncoding];
    [command commit];
    [command waitUntilCompleted];
    if ([command status] != MTLCommandBufferStatusCompleted) {
      report(error, error_size, "dispatch failed", [command error]);
      return -1;
    }
    memcpy(out, [g->output contents], bytes);
    return 0;
  }
}

void tf_metal_close(tf_metal *g) {
  if (!g) return;
  @autoreleasepool {
    [g->output release];
    [g->params release];
    [g->pipeline release];
    [g->queue release];
    [g->device release];
    free(g);
  }
}
