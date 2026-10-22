<template>
  <div v-if="run">
    <a-card>
      <div class="header">
        <a-space align="center">
          <a-button size="small" @click="$router.back()">
            <template #icon><icon-left /></template>
          </a-button>
          <a-typography-title :heading="5" style="margin: 0">运行详情</a-typography-title>
          <StatusTag :status="run.status" />
        </a-space>
        <a-space>
          <a-button v-if="isActive" status="danger" :loading="canceling" @click="doCancel">
            取消运行
          </a-button>
          <a-button @click="loadRun">
            <template #icon><icon-refresh /></template>
            刷新
          </a-button>
        </a-space>
      </div>

      <a-alert v-if="run.error" type="error" style="margin-top: 12px">{{ run.error }}</a-alert>

      <a-descriptions :column="3" size="small" style="margin-top: 16px">
        <a-descriptions-item label="流水线">
          <a-link @click="$router.push(`/pipelines/${run.pipeline_name}`)">
            {{ run.pipeline_name }}
          </a-link>
        </a-descriptions-item>
        <a-descriptions-item label="分支">{{ run.branch || '-' }}</a-descriptions-item>
        <a-descriptions-item label="触发方式">
          {{ run.trigger === 'manual' ? '手动触发' : run.trigger }}
        </a-descriptions-item>
        <a-descriptions-item label="Commit">
          <a-typography-text code>{{ shortSha(run.commit) }}</a-typography-text>
        </a-descriptions-item>
        <a-descriptions-item label="开始时间">{{ fmtTime(run.started_at) }}</a-descriptions-item>
        <a-descriptions-item label="耗时">{{ fmtDuration(run) }}</a-descriptions-item>
        <a-descriptions-item label="提交信息" :span="3">{{ run.message || '-' }}</a-descriptions-item>
        <a-descriptions-item label="Run ID" :span="3">
          <a-typography-text code>{{ run.id }}</a-typography-text>
        </a-descriptions-item>
      </a-descriptions>
    </a-card>

    <a-card title="阶段" style="margin-top: 12px">
      <div v-if="run.stages?.length" class="stage-list">
        <div v-for="(s, i) in run.stages" :key="s.id" class="stage-item">
          <span class="stage-index">{{ i + 1 }}</span>
          <StatusTag :status="s.status" />
          <span class="stage-name">{{ s.name }}</span>
          <a-tag size="small" color="gray">{{ s.type }}</a-tag>
          <span class="stage-meta">
            {{ fmtDurationMs(s.duration_ms) }}
            <template v-if="s.exit_code !== 0"> · exit {{ s.exit_code }}</template>
          </span>
        </div>
      </div>
      <a-empty v-else description="暂无阶段信息" />
    </a-card>

    <a-card title="日志" style="margin-top: 12px">
      <LogViewer :run-id="run.id" @finished="onLogFinished" />
    </a-card>
  </div>

  <a-card v-else>
    <a-skeleton v-if="loading" />
    <a-result v-else status="404" title="运行记录不存在" :subtitle="error">
      <template #extra>
        <a-button type="primary" @click="$router.push('/runs')">返回运行记录</a-button>
      </template>
    </a-result>
  </a-card>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { cancelRun, errorMessage, getRun, type Run } from '../api'
import LogViewer from '../components/LogViewer.vue'
import StatusTag from '../components/StatusTag.vue'
import { fmtDuration, fmtDurationMs, fmtTime, shortSha } from '../utils/format'

const route = useRoute()
const runId = String(route.params.id)

const run = ref<Run | null>(null)
const loading = ref(true)
const canceling = ref(false)
const error = ref('')

let timer: number | undefined

const isActive = computed(
  () => !!run.value && (run.value.status === 'pending' || run.value.status === 'running'),
)

async function loadRun() {
  try {
    run.value = await getRun(runId)
    error.value = ''
    if (isActive.value) startPolling()
    else stopPolling()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function startPolling() {
  if (timer) return
  timer = window.setInterval(loadRun, 2000)
}

function stopPolling() {
  if (timer) {
    window.clearInterval(timer)
    timer = undefined
  }
}

async function doCancel() {
  canceling.value = true
  try {
    await cancelRun(runId)
    Message.success('已请求取消')
    await loadRun()
  } catch (e) {
    Message.error('取消失败: ' + errorMessage(e))
  } finally {
    canceling.value = false
  }
}

function onLogFinished() {
  loadRun()
}

onMounted(loadRun)
onBeforeUnmount(stopPolling)
</script>

<style scoped>
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stage-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.stage-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--color-border-2);
  border-radius: 6px;
  background: var(--color-fill-1);
}

.stage-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--color-fill-3);
  color: var(--color-text-2);
  font-size: 12px;
}

.stage-name {
  font-weight: 500;
}

.stage-meta {
  margin-left: auto;
  color: var(--color-text-3);
  font-size: 12px;
}
</style>
