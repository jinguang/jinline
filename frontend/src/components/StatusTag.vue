<template>
  <a-tag :color="colorMap[status] ?? 'gray'" size="small">{{ textMap[status] ?? status }}</a-tag>
</template>

<script setup lang="ts">
defineProps<{ status: string }>()

const colorMap: Record<string, string> = {
  success: 'green',
  failed: 'red',
  running: 'arcoblue',
  pending: 'orange',
  canceled: 'gray',
}

const textMap: Record<string, string> = {
  success: '成功',
  failed: '失败',
  running: '运行中',
  pending: '排队中',
  canceled: '已取消',
}
</script>
