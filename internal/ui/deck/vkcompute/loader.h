#ifndef TAPLE_LOADER_H
#define TAPLE_LOADER_H
#ifndef VK_NO_PROTOTYPES
#define VK_NO_PROTOTYPES
#endif
#include <vulkan/vulkan.h>

void *tf_loader_open(void);
PFN_vkGetInstanceProcAddr tf_loader_proc(void *loader);
void tf_loader_close(void *loader);
#endif
