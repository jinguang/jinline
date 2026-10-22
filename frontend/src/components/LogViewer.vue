<template>
  <div class="log-viewer">
    <div class="log-toolbar">
      <a-checkbox v-model="autoScroll">自动滚动</a-checkbox>
      <a-space :size="12">
        <span class="log-size">{{ offset }} 字节</span>
        <a-button size="mini" @click="clearLog">清屏</a-button>
      </a-space>
    </div>
    <div ref="box" class="log-box">
      <pre v-if="text">{{ text }}</pre>
      <div v-else class="log-empty">暂无日志</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { fetchLogs, type RunStatus } from '../api'

const props = defineProps<{ runId: string }>()
const emit = defineEmits<{ (e: 'finished', status: RunStatus): void }>()

const text = ref('')
const offset = ref(0)
const autoScroll = ref(true)
const box = ref<HTMLElement>()

let timer: number | undefined
let stopped = false

async function poll() {
  try {
    const chunk = await fetchLogs(props.runId, offset.value)
    if (chunk.text) {
      text.value += chunk.text
      offset.value = chunk.offset
      if (autoScroll.value && box.value) {
        box.value.scrollTop = box.value.scrollHeight
      }
    }
    if (chunk.status !== 'running' && chunk.status !== 'pending') {
      stop()
      emit('finished', chunk.status)
    }
  } catch {
    // 后端暂时不可用时保持轮询
  }
}

function stop() {
  stopped = true
  if (timer) {
    window.clearInterval(timer)
    timer = undefined
  }
}

function clearLog() {
  text.value = ''
}

onMounted(async () => {
  await poll()
  if (stopped) return
  timer = window.setInterval(poll, 1000)
})

onBeforeUnmount(stop)
</script>

<style scoped>
.log-viewer {
  border: 1px solid var(--color-border-2);
  border-radius: 6px;
  overflow: hidden;
  background: #1d2129;
}

.log-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  background: #272e3b;
  color: #c9cdd4;
}

.log-toolbar :deep(.arco-checkbox-label) {
  color: #c9cdd4;
}

.log-size {
  color: #86909c;
  font-size: 12px;
}

.log-box {
  height: 420px;
  overflow: auto;
  padding: 12px;
}

.log-box pre {
  margin: 0;
  color: #e5e6eb;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-empty {
  color: #86909c;
  font-size: 13px;
  padding: 8px;
}
</style>
