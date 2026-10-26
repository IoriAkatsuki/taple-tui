//go:build linux && cgo

#include "loader.h"
#include <dlfcn.h>

void *tf_loader_open(void) {
  return dlopen("libvulkan.so.1", RTLD_NOW | RTLD_LOCAL);
}

PFN_vkGetInstanceProcAddr tf_loader_proc(void *loader) {
  return (PFN_vkGetInstanceProcAddr)dlsym(loader, "vkGetInstanceProcAddr");
}

void tf_loader_close(void *loader) { dlclose(loader); }
