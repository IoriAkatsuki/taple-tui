//go:build darwin && cgo

#include "loader.h"
#include <dlfcn.h>
#include <stddef.h>

void *tf_loader_open(void) {
  const char *paths[] = {
      "@rpath/libvulkan.1.dylib",
      "/opt/homebrew/lib/libvulkan.1.dylib",
      "/usr/local/lib/libvulkan.1.dylib",
      "@rpath/libMoltenVK.dylib",
      "/opt/homebrew/lib/libMoltenVK.dylib",
      "/usr/local/lib/libMoltenVK.dylib",
      "/opt/homebrew/opt/molten-vk/lib/libMoltenVK.dylib",
      "/usr/local/opt/molten-vk/lib/libMoltenVK.dylib",
  };
  for (size_t i = 0; i < sizeof(paths) / sizeof(paths[0]); i++) {
    void *loader = dlopen(paths[i], RTLD_NOW | RTLD_LOCAL);
    if (loader)
      return loader;
  }
  return NULL;
}

PFN_vkGetInstanceProcAddr tf_loader_proc(void *loader) {
  return (PFN_vkGetInstanceProcAddr)dlsym(loader, "vkGetInstanceProcAddr");
}

void tf_loader_close(void *loader) { dlclose(loader); }
