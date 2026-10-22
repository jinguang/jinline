<template>
  <a-card>
    <div class="toolbar">
      <a-space align="center">
        <a-typography-title :heading="5" style="margin: 0">运行记录</a-typography-title>
        <a-select
          v-model="pipelineFilter"
          :options="pipelineOptions"
          allow-clear
          placeholder="全部流水线"
          style="width: 220px"
          @change="onFilterChange"
        />
      </a-space>
      <a-button @click="load">
        <template #icon><icon-refresh /></template>
        刷新
      </a-button>
    </div>

    <a-table
      :data="runs"
      :loading="loading"
      row-key="id"
      :pagination="pagination"
      @page-change="onPageChange"
    >
      <template #columns>
        <a-table-column title="状态" :width="100">
          <template #cell="{ record }">
            <StatusTag :status="record.status" />
          </template>
        </a-table-column>
        <a-table-column title="流水线" :width="180">
          <template #cell="{ record }">
            <a-link @click="$router.push(`/pipelines/${record.pipeline_name}`)">
              {{ record.pipeline_name }}
            </a-link>
          </template>
        </a-table-column>
        <a-table-column title="分支" :width="120">
          <template #cell="{ record }">{{ record.branch || '-' }}</template>
        </a-table-column>
        <a-table-column title="Commit" :width="120">
          <template #cell="{ record }">
            <a-typography-text code>{{ shortSha(record.commit) }}</a-typography-text>
          </template>
        </a-table-column>
        <a-table-column title="提交信息">
          <template #cell="{ record }">
            <span class="message">{{ record.message || '-' }}</span>
          </template>
        </a-table-column>
        <a-table-column title="开始时间" :width="170">
          <template #cell="{ record }">{{ fmtTime(record.started_at) }}</template>
        </a-table-column>
        <a-table-column title="耗时" :width="90">
          <template #cell="{ record }">{{ fmtDuration(record) }}</template>
        </a-table-column>
        <a-table-column title="操作" :width="80">
          <template #cell="{ record }">
            <a-link @click="$router.push(`/runs/${record.id}`)">详情</a-link>
          </template>
        </a-table-column>
      </template>
      <template #empty>
        <a-empty description="还没有运行记录" />
      </template>
    </a-table>
  </a-card>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import {
  errorMessage,
  listPipelines,
  listRuns,
  type PipelineSummary,
  type Run,
} from '../api'
import StatusTag from '../components/StatusTag.vue'
import { fmtDuration, fmtTime, shortSha } from '../utils/format'

const runs = ref<Run[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const pipelineFilter = ref<string | undefined>(undefined)
const pipelines = ref<PipelineSummary[]>([])

let timer: number | undefined

const pipelineOptions = computed(() =>
  pipelines.value.map((p) => ({ label: p.name, value: p.name })),
)

const pagination = computed(() => ({
  total: total.value,
  current: page.value,
  pageSize: pageSize.value,
  showTotal: true,
}))

async function load() {
  loading.value = true
  try {
    const res = await listRuns({
      pipeline: pipelineFilter.value,
      page: page.value,
      page_size: pageSize.value,
    })
    runs.value = res.items ?? []
    total.value = res.total ?? 0
  } catch (e) {
    Message.error('加载失败: ' + errorMessage(e))
  } finally {
    loading.value = false
  }
}

function onPageChange(p: number) {
  page.value = p
  load()
}

function onFilterChange() {
  page.value = 1
  load()
}

onMounted(async () => {
  load()
  try {
    pipelines.value = await listPipelines()
  } catch {
    // 下拉选项加载失败不影响列表
  }
  timer = window.setInterval(load, 5000)
})

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.message {
  color: var(--color-text-2);
  font-size: 12px;
}
</style>
