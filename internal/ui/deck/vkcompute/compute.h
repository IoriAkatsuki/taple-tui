#ifndef TAPLE_COMPUTE_H
#define TAPLE_COMPUTE_H
#include <stddef.h>
#include <stdint.h>
typedef struct tf_gpu tf_gpu;
tf_gpu *tf_gpu_open(const uint32_t *code, size_t bytes, char *error,
                    size_t error_size);
const char *tf_gpu_name(tf_gpu *g);
int tf_gpu_run(tf_gpu *g, const float *params, float *out, uint32_t pixels);
void tf_gpu_close(tf_gpu *g);
#endif
