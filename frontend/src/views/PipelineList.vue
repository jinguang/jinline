<template>
  <a-card>
    <div class="toolbar">
      <div>
        <a-typography-title :heading="5" style="margin: 0">流水线</a-typography-title>
        <a-typography-text type="secondary">从 Git 拉取代码，编译并构建容器镜像</a-typography-text>
      </div>
      <a-space>
        <a-button @click="load">
          <template #icon><icon-refresh /></template>
          刷新
        </a-button>
        <a-button type="primary" @click="$router.push('/pipelines/new')">
          <template #icon><icon-plus /></template>
          新建流水线
        </a-button>
      </a-space>
    </div>

    <a-table :data="pipelines" :loading="loading" :pagination="false" row-key="name">
      <template #columns>
        <a-table-column title="流水线" :width="200">
          <template #cell="{ record }">
            <a-link @click="$router.push(`/pipelines/${record.name}`)">{{ record.name }}</a-link>
            <div v-if="record.description" class="desc">{{ record.description }}</div>
          </template>
        </a-table-column>

        <a-table-column title="Git 仓库">
          <template #cell="{ record }">
            <div class="repo">{{ record.git.url }}</div>
            <a-tag v-if="record.git.branch" size="small" color="gray">{{ record.git.branch }}</a-tag>
          </template>
        </a-table-column>

        <a-table-column title="阶段" :width="220">
          <template #cell="{ record }">
            <a-space wrap :size="4">
              <a-tag v-for="stage in record.stages" :key="stage" size="small">{{ stage }}</a-tag>
            </a-space>
          </template>
        </a-table-column>

        <a-table-column title="最近运行" :width="200">
          <template #cell="{ record }">
            <template v-if="record.last_run">
              <a-space :size="6">
                <StatusTag :status="record.last_run.status" />
                <a-link @click="$router.push(`/runs/${record.last_run.id}`)">日志</a-link>
              </a-space>
              <div class="time">{{ fmtTime(record.last_run.created_at) }}</div>
            </template>
            <span v-else class="time">从未运行</span>
          </template>
        </a-table-column>

        <a-table-column title="操作" :width="220">
          <template #cell="{ record }">
            <a-space :size="8">
              <a-button size="small" type="primary" :loading="running === record.name" @click="run(record)">
                运行
              </a-button>
              <a-button size="small" @click="$router.push(`/pipelines/${record.name}/edit`)">编辑</a-button>
              <a-popconfirm content="确认删除该流水线配置？运行记录会保留。" type="warning" @ok="remove(record)">
                <a-button size="small" status="danger">删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </a-table-column>
      </template>

      <template #empty>
        <a-empty description="还没有流水线，点击右上角新建" />
      </template>
    </a-table>
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { deletePipeline, errorMessage, listPipelines, triggerRun, type PipelineSummary } from '../api'
import StatusTag from '../components/StatusTag.vue'
import { fmtTime } from '../utils/format'

const router = useRouter()
const pipelines = ref<PipelineSummary[]>([])
const loading = ref(false)
const running = ref('')

async function load() {
  loading.value = true
  try {
    pipelines.value = await listPipelines()
  } catch (e) {
    Message.error('加载失败: ' + errorMessage(e))
  } finally {
    loading.value = false
  }
}

async function run(p: PipelineSummary) {
  running.value = p.name
  try {
    const run = await triggerRun(p.name)
    Message.success('已触发运行')
    router.push(`/runs/${run.id}`)
  } catch (e) {
    Message.error('触发失败: ' + errorMessage(e))
  } finally {
    running.value = ''
  }
}

async function remove(p: PipelineSummary) {
  try {
    await deletePipeline(p.name)
    Message.success('已删除')
    load()
  } catch (e) {
    Message.error('删除失败: ' + errorMessage(e))
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.desc {
  color: var(--color-text-3);
  font-size: 12px;
  margin-top: 2px;
}

.repo {
  font-size: 12px;
  word-break: break-all;
  margin-bottom: 2px;
}

.time {
  color: var(--color-text-3);
  font-size: 12px;
  margin-top: 2px;
}
</style>
