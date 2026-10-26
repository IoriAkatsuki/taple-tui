//go:build (linux || windows || darwin) && cgo

#define VK_NO_PROTOTYPES
#include "compute.h"
#include "loader.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define GLOBAL_FUNCTIONS(X) \
  X(vkCreateInstance) X(vkEnumerateInstanceExtensionProperties)
#define INSTANCE_FUNCTIONS(X) \
  X(vkDestroyInstance) X(vkEnumeratePhysicalDevices) \
  X(vkGetPhysicalDeviceProperties) X(vkGetPhysicalDeviceQueueFamilyProperties) \
  X(vkGetPhysicalDeviceMemoryProperties) X(vkEnumerateDeviceExtensionProperties) \
  X(vkCreateDevice) X(vkGetDeviceProcAddr)
#define DEVICE_FUNCTIONS(X) \
  X(vkDestroyDevice) X(vkGetDeviceQueue) X(vkDeviceWaitIdle) \
  X(vkCreateBuffer) X(vkDestroyBuffer) X(vkGetBufferMemoryRequirements) \
  X(vkAllocateMemory) X(vkFreeMemory) X(vkBindBufferMemory) \
  X(vkMapMemory) X(vkUnmapMemory) X(vkCreateShaderModule) \
  X(vkDestroyShaderModule) X(vkCreateDescriptorSetLayout) \
  X(vkDestroyDescriptorSetLayout) X(vkCreatePipelineLayout) \
  X(vkDestroyPipelineLayout) X(vkCreateComputePipelines) X(vkDestroyPipeline) \
  X(vkCreateDescriptorPool) X(vkDestroyDescriptorPool) X(vkAllocateDescriptorSets) \
  X(vkUpdateDescriptorSets) X(vkCreateCommandPool) X(vkDestroyCommandPool) \
  X(vkAllocateCommandBuffers) X(vkResetCommandBuffer) X(vkBeginCommandBuffer) \
  X(vkEndCommandBuffer) X(vkCmdBindPipeline) X(vkCmdBindDescriptorSets) \
  X(vkCmdDispatch) X(vkCmdPipelineBarrier) X(vkCreateFence) X(vkDestroyFence) \
  X(vkResetFences) X(vkQueueSubmit) X(vkWaitForFences)

struct tf_gpu {
  void *loader;
  PFN_vkGetInstanceProcAddr vkGetInstanceProcAddr;
#define FIELD(name) PFN_##name name;
  GLOBAL_FUNCTIONS(FIELD)
  INSTANCE_FUNCTIONS(FIELD)
  DEVICE_FUNCTIONS(FIELD)
#undef FIELD
  VkInstance instance;
  VkPhysicalDevice physical;
  VkDevice device;
  VkQueue queue;
  VkDescriptorSetLayout set_layout;
  VkPipelineLayout layout;
  VkShaderModule shader;
  VkPipeline pipeline;
  VkDescriptorPool descriptor_pool;
  VkDescriptorSet set;
  VkCommandPool command_pool;
  VkCommandBuffer command;
  VkFence fence;
  VkBuffer buffer;
  VkDeviceMemory memory;
  void *mapped;
  VkDeviceSize capacity, output_offset;
  uint32_t max_range, max_groups;
  int failed;
  char name[VK_MAX_PHYSICAL_DEVICE_NAME_SIZE];
};

static void free_buffer(tf_gpu *g) {
  if (g->mapped)
    g->vkUnmapMemory(g->device, g->memory);
  if (g->buffer)
    g->vkDestroyBuffer(g->device, g->buffer, NULL);
  if (g->memory)
    g->vkFreeMemory(g->device, g->memory, NULL);
  g->mapped = NULL;
  g->buffer = VK_NULL_HANDLE;
  g->memory = VK_NULL_HANDLE;
  g->capacity = 0;
}

void tf_gpu_close(tf_gpu *g) {
  if (!g)
    return;
  if (g->device) {
    if (g->vkDeviceWaitIdle)
      g->vkDeviceWaitIdle(g->device);
    free_buffer(g);
    if (g->fence)
      g->vkDestroyFence(g->device, g->fence, NULL);
    if (g->command_pool)
      g->vkDestroyCommandPool(g->device, g->command_pool, NULL);
    if (g->descriptor_pool)
      g->vkDestroyDescriptorPool(g->device, g->descriptor_pool, NULL);
    if (g->pipeline)
      g->vkDestroyPipeline(g->device, g->pipeline, NULL);
    if (g->shader)
      g->vkDestroyShaderModule(g->device, g->shader, NULL);
    if (g->layout)
      g->vkDestroyPipelineLayout(g->device, g->layout, NULL);
    if (g->set_layout)
      g->vkDestroyDescriptorSetLayout(g->device, g->set_layout, NULL);
    g->vkDestroyDevice(g->device, NULL);
  }
  if (g->instance && g->vkDestroyInstance)
    g->vkDestroyInstance(g->instance, NULL);
  if (g->loader)
    tf_loader_close(g->loader);
  free(g);
}

static VkResult extensions(tf_gpu *g, VkPhysicalDevice physical,
                           VkExtensionProperties **props, uint32_t *count) {
  *props = NULL;
  VkResult result =
      physical
          ? g->vkEnumerateDeviceExtensionProperties(physical, NULL, count, NULL)
          : g->vkEnumerateInstanceExtensionProperties(NULL, count, NULL);
  if (result != VK_SUCCESS || !*count)
    return result;
  *props = calloc(*count, sizeof(**props));
  if (!*props)
    return VK_ERROR_OUT_OF_HOST_MEMORY;
  result = physical
               ? g->vkEnumerateDeviceExtensionProperties(physical, NULL, count,
                                                         *props)
               : g->vkEnumerateInstanceExtensionProperties(NULL, count, *props);
  if (result != VK_SUCCESS) {
    free(*props);
    *props = NULL;
  }
  return result;
}

static int has_extension(const VkExtensionProperties *props, uint32_t count,
                         const char *name) {
  for (uint32_t i = 0; i < count; i++)
    if (strcmp(props[i].extensionName, name) == 0)
      return 1;
  return 0;
}

#define TRY(call)                                                              \
  do {                                                                         \
    result = (call);                                                           \
    if (result != VK_SUCCESS) {                                                \
      snprintf(error, error_size, "%s: VkResult %d", #call, result);           \
      goto fail;                                                               \
    }                                                                          \
  } while (0)
tf_gpu *tf_gpu_open(const uint32_t *code, size_t bytes, char *error,
                    size_t error_size) {
  VkResult result;
  tf_gpu *g = calloc(1, sizeof(*g));
  if (!g) {
    snprintf(error, error_size, "Vulkan allocation failed");
    return NULL;
  }
  g->loader = tf_loader_open();
  if (!g->loader) {
    snprintf(error, error_size, "Vulkan loader unavailable");
    goto fail;
  }
  g->vkGetInstanceProcAddr = tf_loader_proc(g->loader);
  if (!g->vkGetInstanceProcAddr) {
    snprintf(error, error_size, "missing vkGetInstanceProcAddr");
    goto fail;
  }
#define LOAD(name, address)                                                    \
  do {                                                                         \
    PFN_##name proc = (PFN_##name)(address);                                   \
    if (!proc) {                                                               \
      snprintf(error, error_size, "missing %s", #name);                        \
      goto fail;                                                               \
    }                                                                          \
    g->name = proc;                                                            \
  } while (0);
#define LOAD_GLOBAL(name)                                                      \
  LOAD(name, g->vkGetInstanceProcAddr(VK_NULL_HANDLE, #name))
  GLOBAL_FUNCTIONS(LOAD_GLOBAL)
#undef LOAD_GLOBAL
  uint32_t extension_count = 0;
  VkExtensionProperties *props = NULL;
  TRY(extensions(g, VK_NULL_HANDLE, &props, &extension_count));
  const char *instance_extensions[3];
  uint32_t instance_extension_count = 0;
  int portability = has_extension(
      props, extension_count, VK_KHR_PORTABILITY_ENUMERATION_EXTENSION_NAME);
  int properties2 =
      has_extension(props, extension_count,
                    VK_KHR_GET_PHYSICAL_DEVICE_PROPERTIES_2_EXTENSION_NAME);
#ifdef __APPLE__
  int layer_settings =
      has_extension(props, extension_count, VK_EXT_LAYER_SETTINGS_EXTENSION_NAME);
#endif
  free(props);
#ifdef __APPLE__
  if (!layer_settings) {
    snprintf(error, error_size, "MoltenVK requires VK_EXT_layer_settings for safe initialization");
    goto fail;
  }
#endif
  if (portability)
    instance_extensions[instance_extension_count++] =
        VK_KHR_PORTABILITY_ENUMERATION_EXTENSION_NAME;
  if (properties2)
    instance_extensions[instance_extension_count++] =
        VK_KHR_GET_PHYSICAL_DEVICE_PROPERTIES_2_EXTENSION_NAME;
#ifdef __APPLE__
  // Two storage buffers need no argument encoders, whose probe aborts on Paravirtual Metal.
  VkBool32 argument_buffers = VK_FALSE;
  VkLayerSettingEXT setting = {
      .pLayerName = "MoltenVK",
      .pSettingName = "MVK_CONFIG_USE_METAL_ARGUMENT_BUFFERS",
      .type = VK_LAYER_SETTING_TYPE_BOOL32_EXT,
      .valueCount = 1,
      .pValues = &argument_buffers};
  VkLayerSettingsCreateInfoEXT settings = {
      .sType = VK_STRUCTURE_TYPE_LAYER_SETTINGS_CREATE_INFO_EXT,
      .settingCount = 1,
      .pSettings = &setting};
  instance_extensions[instance_extension_count++] = VK_EXT_LAYER_SETTINGS_EXTENSION_NAME;
#endif
  VkApplicationInfo app = {.sType = VK_STRUCTURE_TYPE_APPLICATION_INFO,
                           .pApplicationName = "tapefox",
                           .apiVersion = VK_API_VERSION_1_0};
  VkInstanceCreateInfo instance = {
      .sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
#ifdef __APPLE__
      .pNext = &settings,
#endif
      .flags =
          portability ? VK_INSTANCE_CREATE_ENUMERATE_PORTABILITY_BIT_KHR : 0,
      .enabledExtensionCount = instance_extension_count,
      .ppEnabledExtensionNames = instance_extensions,
      .pApplicationInfo = &app};
  TRY(g->vkCreateInstance(&instance, NULL, &g->instance));
#define LOAD_INSTANCE(name)                                                    \
  LOAD(name, g->vkGetInstanceProcAddr(g->instance, #name))
  INSTANCE_FUNCTIONS(LOAD_INSTANCE)
  // Keep a cleanup entry point even if loading device dispatch fails partway.
  LOAD_INSTANCE(vkDestroyDevice)
#undef LOAD_INSTANCE
  uint32_t count = 0;
  TRY(g->vkEnumeratePhysicalDevices(g->instance, &count, NULL));
  VkPhysicalDevice *devices = calloc(count, sizeof(*devices));
  if (!devices || !count) {
    free(devices);
    snprintf(error, error_size, "no Vulkan physical GPU");
    goto fail;
  }
  result = g->vkEnumeratePhysicalDevices(g->instance, &count, devices);
  if (result != VK_SUCCESS) {
    free(devices);
    snprintf(error, error_size, "cannot enumerate Vulkan GPUs");
    goto fail;
  }
  uint32_t family = 0;
  int best = 0;
  for (uint32_t i = 0; i < count; i++) {
    VkPhysicalDeviceProperties props;
    g->vkGetPhysicalDeviceProperties(devices[i], &props);
    int score = props.deviceType == VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU ? 2
                : props.deviceType == VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU
                    ? 1
                    : 0;
    if (score <= best)
      continue; // Never report lavapipe/software Vulkan as GPU acceleration.
    uint32_t n = 0;
    g->vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &n, NULL);
    VkQueueFamilyProperties *q = calloc(n, sizeof(*q));
    if (!q)
      continue;
    g->vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &n, q);
    for (uint32_t j = 0; j < n; j++)
      if (q[j].queueCount && (q[j].queueFlags & VK_QUEUE_COMPUTE_BIT)) {
        g->physical = devices[i];
        family = j;
        best = score;
        memcpy(g->name, props.deviceName, sizeof(g->name));
        g->max_range = props.limits.maxStorageBufferRange;
        g->max_groups = props.limits.maxComputeWorkGroupCount[0];
        VkDeviceSize alignment = props.limits.minStorageBufferOffsetAlignment;
        g->output_offset = (512 + alignment - 1) / alignment * alignment;
        break;
      }
    free(q);
  }
  free(devices);
  if (!g->physical) {
    snprintf(error, error_size, "no hardware Vulkan compute device");
    goto fail;
  }
  TRY(extensions(g, g->physical, &props, &extension_count));
  const char *subset = "VK_KHR_portability_subset";
  int portability_subset = has_extension(props, extension_count, subset);
  free(props);
  if (portability_subset && !properties2) {
    snprintf(
        error, error_size,
        "Vulkan portability requires VK_KHR_get_physical_device_properties2");
    goto fail;
  }
  float priority = 1;
  VkDeviceQueueCreateInfo qi = {.sType =
                                    VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,
                                .queueFamilyIndex = family,
                                .queueCount = 1,
                                .pQueuePriorities = &priority};
  VkDeviceCreateInfo di = {.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,
                           .enabledExtensionCount = portability_subset ? 1 : 0,
                           .ppEnabledExtensionNames = &subset,
                           .queueCreateInfoCount = 1,
                           .pQueueCreateInfos = &qi};
  TRY(g->vkCreateDevice(g->physical, &di, NULL, &g->device));
#define LOAD_DEVICE(name) LOAD(name, g->vkGetDeviceProcAddr(g->device, #name))
  DEVICE_FUNCTIONS(LOAD_DEVICE)
#undef LOAD_DEVICE
#undef LOAD
  g->vkGetDeviceQueue(g->device, family, 0, &g->queue);
  VkDescriptorSetLayoutBinding bindings[2] = {
      {.binding = 0,
       .descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,
       .descriptorCount = 1,
       .stageFlags = VK_SHADER_STAGE_COMPUTE_BIT},
      {.binding = 1,
       .descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,
       .descriptorCount = 1,
       .stageFlags = VK_SHADER_STAGE_COMPUTE_BIT}};
  VkDescriptorSetLayoutCreateInfo sli = {
      .sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO,
      .bindingCount = 2,
      .pBindings = bindings};
  TRY(g->vkCreateDescriptorSetLayout(g->device, &sli, NULL, &g->set_layout));
  VkPipelineLayoutCreateInfo pli = {
      .sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO,
      .setLayoutCount = 1,
      .pSetLayouts = &g->set_layout};
  TRY(g->vkCreatePipelineLayout(g->device, &pli, NULL, &g->layout));
  VkShaderModuleCreateInfo sm = {
      .sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO,
      .codeSize = bytes,
      .pCode = code};
  TRY(g->vkCreateShaderModule(g->device, &sm, NULL, &g->shader));
  VkComputePipelineCreateInfo cp = {
      .sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO,
      .layout = g->layout,
      .stage = {.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO,
                .stage = VK_SHADER_STAGE_COMPUTE_BIT,
                .module = g->shader,
                .pName = "main"}};
  TRY(g->vkCreateComputePipelines(g->device, VK_NULL_HANDLE, 1, &cp, NULL,
                                  &g->pipeline));
  VkDescriptorPoolSize ps = {.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,
                             .descriptorCount = 2};
  VkDescriptorPoolCreateInfo dpi = {
      .sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
      .maxSets = 1,
      .poolSizeCount = 1,
      .pPoolSizes = &ps};
  TRY(g->vkCreateDescriptorPool(g->device, &dpi, NULL, &g->descriptor_pool));
  VkDescriptorSetAllocateInfo sa = {
      .sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
      .descriptorPool = g->descriptor_pool,
      .descriptorSetCount = 1,
      .pSetLayouts = &g->set_layout};
  TRY(g->vkAllocateDescriptorSets(g->device, &sa, &g->set));
  VkCommandPoolCreateInfo pool = {
      .sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,
      .flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT,
      .queueFamilyIndex = family};
  TRY(g->vkCreateCommandPool(g->device, &pool, NULL, &g->command_pool));
  VkCommandBufferAllocateInfo ca = {
      .sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
      .commandPool = g->command_pool,
      .level = VK_COMMAND_BUFFER_LEVEL_PRIMARY,
      .commandBufferCount = 1};
  TRY(g->vkAllocateCommandBuffers(g->device, &ca, &g->command));
  VkFenceCreateInfo fi = {.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO};
  TRY(g->vkCreateFence(g->device, &fi, NULL, &g->fence));
  return g;
fail:
  tf_gpu_close(g);
  return NULL;
}
#undef TRY

const char *tf_gpu_name(tf_gpu *g) { return g->name; }

static VkResult resize_buffer(tf_gpu *g, VkDeviceSize bytes) {
  if (g->capacity >= bytes)
    return VK_SUCCESS;
  free_buffer(g);
  VkBufferCreateInfo bi = {.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO,
                           .size = bytes,
                           .usage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                           .sharingMode = VK_SHARING_MODE_EXCLUSIVE};
  VkResult r = g->vkCreateBuffer(g->device, &bi, NULL, &g->buffer);
  if (r)
    return r;
  VkMemoryRequirements req;
  g->vkGetBufferMemoryRequirements(g->device, g->buffer, &req);
  VkPhysicalDeviceMemoryProperties props;
  g->vkGetPhysicalDeviceMemoryProperties(g->physical, &props);
  uint32_t index = UINT32_MAX;
  for (uint32_t i = 0; i < props.memoryTypeCount; i++)
    if ((req.memoryTypeBits & (1u << i)) &&
        (props.memoryTypes[i].propertyFlags &
         (VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
          VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)) ==
            (VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
             VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)) {
      if (index == UINT32_MAX)
        index = i;
      // Output is read every frame: uncached/write-combined memory makes
      // readback dominate rendering.
      if (props.memoryTypes[i].propertyFlags &
          VK_MEMORY_PROPERTY_HOST_CACHED_BIT) {
        index = i;
        break;
      }
    }
  if (index == UINT32_MAX)
    return VK_ERROR_FEATURE_NOT_PRESENT;
  VkMemoryAllocateInfo ai = {.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
                             .allocationSize = req.size,
                             .memoryTypeIndex = index};
  r = g->vkAllocateMemory(g->device, &ai, NULL, &g->memory);
  if (r)
    return r;
  r = g->vkBindBufferMemory(g->device, g->buffer, g->memory, 0);
  if (r)
    return r;
  r = g->vkMapMemory(g->device, g->memory, 0, req.size, 0, &g->mapped);
  if (r)
    return r;
  g->capacity = bytes;
  VkDescriptorBufferInfo buffers[2] = {
      {.buffer = g->buffer, .offset = 0, .range = 512},
      {.buffer = g->buffer,
       .offset = g->output_offset,
       .range = bytes - g->output_offset}};
  VkWriteDescriptorSet writes[2];
  memset(writes, 0, sizeof(writes));
  for (int i = 0; i < 2; i++) {
    writes[i].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[i].dstSet = g->set;
    writes[i].dstBinding = i;
    writes[i].descriptorCount = 1;
    writes[i].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[i].pBufferInfo = &buffers[i];
  }
  g->vkUpdateDescriptorSets(g->device, 2, writes, 0, NULL);
  return VK_SUCCESS;
}

int tf_gpu_run(tf_gpu *g, const float *params, float *out, uint32_t pixels) {
  if (g->failed)
    return g->failed;
  VkDeviceSize bytes = (VkDeviceSize)pixels * 16;
  uint32_t groups = (pixels + 63) / 64;
  if (!pixels || bytes > g->max_range || groups > g->max_groups)
    return VK_ERROR_OUT_OF_DEVICE_MEMORY;
#define RUN(call)                                                              \
  do {                                                                         \
    VkResult r = (call);                                                       \
    if (r != VK_SUCCESS) {                                                     \
      g->failed = r;                                                           \
      return r;                                                                \
    }                                                                          \
  } while (0)
  RUN(resize_buffer(g, g->output_offset + bytes));
  memcpy(g->mapped, params, 512);
  RUN(g->vkResetCommandBuffer(g->command, 0));
  VkCommandBufferBeginInfo begin = {
      .sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO,
      .flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT};
  RUN(g->vkBeginCommandBuffer(g->command, &begin));
  g->vkCmdBindPipeline(g->command, VK_PIPELINE_BIND_POINT_COMPUTE, g->pipeline);
  g->vkCmdBindDescriptorSets(g->command, VK_PIPELINE_BIND_POINT_COMPUTE,
                             g->layout, 0, 1, &g->set, 0, NULL);
  g->vkCmdDispatch(g->command, groups, 1, 1);
  VkMemoryBarrier barrier = {.sType = VK_STRUCTURE_TYPE_MEMORY_BARRIER,
                             .srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT,
                             .dstAccessMask = VK_ACCESS_HOST_READ_BIT};
  g->vkCmdPipelineBarrier(g->command, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, 0, 1, &barrier, 0, NULL,
                          0, NULL);
  RUN(g->vkEndCommandBuffer(g->command));
  RUN(g->vkResetFences(g->device, 1, &g->fence));
  VkSubmitInfo submit = {.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO,
                         .commandBufferCount = 1,
                         .pCommandBuffers = &g->command};
  RUN(g->vkQueueSubmit(g->queue, 1, &submit, g->fence));
  RUN(g->vkWaitForFences(g->device, 1, &g->fence, VK_TRUE, 100000000));
  memcpy(out, (char *)g->mapped + g->output_offset, (size_t)bytes);
  return VK_SUCCESS;
#undef RUN
}
