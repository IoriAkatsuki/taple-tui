// go:build linux && cgo

#define VK_NO_PROTOTYPES
#include "compute.h"
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <vulkan/vulkan.h>

#define FUNCTIONS(X)                                                             \
  X(vkCreateInstance)                                                            \
  X(vkDestroyInstance) X(vkEnumeratePhysicalDevices) X(                          \
      vkGetPhysicalDeviceProperties) X(vkGetPhysicalDeviceQueueFamilyProperties) \
      X(vkGetPhysicalDeviceMemoryProperties) X(vkCreateDevice) X(                \
          vkDestroyDevice) X(vkGetDeviceQueue) X(vkDeviceWaitIdle)               \
          X(vkCreateBuffer) X(vkDestroyBuffer) X(                                \
              vkGetBufferMemoryRequirements) X(vkAllocateMemory)                 \
              X(vkFreeMemory) X(vkBindBufferMemory) X(vkMapMemory) X(            \
                  vkUnmapMemory) X(vkCreateShaderModule)                         \
                  X(vkDestroyShaderModule) X(vkCreateDescriptorSetLayout) X(     \
                      vkDestroyDescriptorSetLayout) X(vkCreatePipelineLayout)    \
                      X(vkDestroyPipelineLayout) X(vkCreateComputePipelines) X(  \
                          vkDestroyPipeline) X(vkCreateDescriptorPool)           \
                          X(vkDestroyDescriptorPool) X(                          \
                              vkAllocateDescriptorSets)                          \
                              X(vkUpdateDescriptorSets) X(vkCreateCommandPool)   \
                                  X(vkDestroyCommandPool) X(                     \
                                      vkAllocateCommandBuffers)                  \
                                      X(vkResetCommandBuffer) X(                 \
                                          vkBeginCommandBuffer)                  \
                                          X(vkEndCommandBuffer) X(               \
                                              vkCmdBindPipeline)                 \
                                              X(vkCmdBindDescriptorSets) X(      \
                                                  vkCmdDispatch)                 \
                                                  X(vkCmdPipelineBarrier) X(     \
                                                      vkCreateFence)             \
                                                      X(vkDestroyFence) X(       \
                                                          vkResetFences)         \
                                                          X(vkQueueSubmit) X(    \
                                                              vkWaitForFences)

struct tf_gpu {
  void *loader;
#define FIELD(name) PFN_##name name;
  FUNCTIONS(FIELD)
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
  if (g->instance)
    g->vkDestroyInstance(g->instance, NULL);
  if (g->loader)
    dlclose(g->loader);
  free(g);
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
  g->loader = dlopen("libvulkan.so.1", RTLD_NOW | RTLD_LOCAL);
  if (!g->loader) {
    snprintf(error, error_size, "libvulkan.so.1 unavailable");
    goto fail;
  }
#define LOAD(name)                                                             \
  g->name = (PFN_##name)dlsym(g->loader, #name);                               \
  if (!g->name) {                                                              \
    snprintf(error, error_size, "missing %s", #name);                          \
    goto fail;                                                                 \
  }
  FUNCTIONS(LOAD)
#undef LOAD
  VkApplicationInfo app = {.sType = VK_STRUCTURE_TYPE_APPLICATION_INFO,
                           .pApplicationName = "tapefox",
                           .apiVersion = VK_API_VERSION_1_0};
  VkInstanceCreateInfo instance = {.sType =
                                       VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
                                   .pApplicationInfo = &app};
  TRY(g->vkCreateInstance(&instance, NULL, &g->instance));
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
  float priority = 1;
  VkDeviceQueueCreateInfo qi = {.sType =
                                    VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,
                                .queueFamilyIndex = family,
                                .queueCount = 1,
                                .pQueuePriorities = &priority};
  VkDeviceCreateInfo di = {.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,
                           .queueCreateInfoCount = 1,
                           .pQueueCreateInfos = &qi};
  TRY(g->vkCreateDevice(g->physical, &di, NULL, &g->device));
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
