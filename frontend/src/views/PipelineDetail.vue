<template>
  <div v-if="detail">
    <a-card>
      <div class="header">
        <div>
          <a-space align="center">
            <a-typography-title :heading="5" style="margin: 0">{{ detail.name }}</a-typography-title>
            <a-button size="mini" @click="$router.push(`/pipelines/${detail.name}/edit`)">编辑配置</a-button>
          </a-space>
          <a-typography-paragraph v-if="detail.description" type="secondary" style="margin: 6px 0 0">
            {{ detail.description }}
          </a-typography-paragraph>
        </div>
        <a-space>
          <a-button :loading="running" type="primary" @click="run">
            <template #icon><icon-play-circle /></template>
            运行
          </a-button>
          <a-popconfirm content="确认删除该流水线配置？运行记录会保留。" type="warning" @ok="remove">
            <a-button status="danger">删除</a-button>
          </a-popconfirm>
        </a-space>
      </div>

      <a-descriptions :column="2" size="small" style="margin-top: 16px">
        <a-descriptions-item label="Git 仓库">{{ detail.git.url }}</a-descriptions-item>
        <a-descriptions-item label="分支">{{ detail.git.branch || '仓库默认分支' }}</a-descriptions-item>
        <a-descriptions-item label="工作目录">{{ detail.workdir }}</a-descriptions-item>
        <a-descriptions-item label="阶段">
          <a-space wrap :size="4">
            <a-tag v-for="s in detail.stages" :key="s.name" size="small">{{ s.name }}</a-tag>
          </a-space>
        </a-descriptions-item>
      </a-descriptions>
    </a-card>

    <a-card title="运行历史" style="margin-top: 12px">
      <a-table :data="detail.runs" :loading="loading" :pagination="false" row-key="id">
        <template #columns>
          <a-table-column title="状态" :width="100">
            <template #cell="{ record }">
              <StatusTag :status="record.status" />
            </template>
          </a-table-column>
          <a-table-column title="分支" :width="140">
            <template #cell="{ record }">{{ record.branch || '-' }}</template>
          </a-table-column>
          <a-table-column title="Commit" :width="160">
            <template #cell="{ record }">
              <a-typography-text code>{{ shortSha(record.commit) }}</a-typography-text>
            </template>
          </a-table-column>
          <a-table-column title="提交信息">
            <template #cell="{ record }">
              <span class="message">{{ record.message || '-' }}</span>
            </template>
          </a-table-column>
          <a-table-column title="开始时间" :width="180">
            <template #cell="{ record }">{{ fmtTime(record.started_at) }}</template>
          </a-table-column>
          <a-table-column title="耗时" :width="90">
            <template #cell="{ record }">{{ fmtDuration(record) }}</template>
          </a-table-column>
          <a-table-column title="操作" :width="90">
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
  </div>

  <a-card v-else>
    <a-skeleton v-if="loading" />
    <a-result v-else status="404" title="流水线不存在" :subtitle="error">
      <template #extra>
        <a-button type="primary" @click="$router.push('/pipelines')">返回列表</a-button>
      </template>
    </a-result>
  </a-card>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import {
  deletePipeline,
  errorMessage,
  getPipeline,
  triggerRun,
  type PipelineDetail,
} from '../api'
import StatusTag from '../components/StatusTag.vue'
import { fmtDuration, fmtTime, shortSha } from '../utils/format'

const route = useRoute()
const router = useRouter()

const name = String(route.params.name)
const detail = ref<PipelineDetail | null>(null)
const loading = ref(false)
const running = ref(false)
const error = ref('')

let timer: number | undefined

async function load() {
  loading.value = true
  try {
    detail.value = await getPipeline(name)
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

async function run() {
  running.value = true
  try {
    const r = await triggerRun(name)
    Message.success('已触发运行')
    router.push(`/runs/${r.id}`)
  } catch (e) {
    Message.error('触发失败: ' + errorMessage(e))
  } finally {
    running.value = false
  }
}

async function remove() {
  try {
    await deletePipeline(name)
    Message.success('已删除')
    router.push('/pipelines')
  } catch (e) {
    Message.error('删除失败: ' + errorMessage(e))
  }
}

onMounted(() => {
  load()
  timer = window.setInterval(load, 8000)
})

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<style scoped>
.header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.message {
  color: var(--color-text-2);
  font-size: 12px;
}
</style>
