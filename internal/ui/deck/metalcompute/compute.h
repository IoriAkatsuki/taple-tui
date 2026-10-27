#ifndef TAPLE_METAL_COMPUTE_H
#define TAPLE_METAL_COMPUTE_H
#include <stddef.h>
#include <stdint.h>

typedef struct tf_metal tf_metal;
tf_metal *tf_metal_open(const char *source, char *error, size_t error_size);
const char *tf_metal_name(tf_metal *g);
int tf_metal_run(tf_metal *g, const float *params, float *out, uint32_t pixels,
                 char *error, size_t error_size);
void tf_metal_close(tf_metal *g);
#endif
