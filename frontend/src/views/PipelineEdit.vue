<template>
  <a-card :title="isEdit ? `编辑流水线 / ${name}` : '新建流水线'">
    <a-alert v-if="error" type="error" style="margin-bottom: 12px">{{ error }}</a-alert>

    <a-textarea
      v-model="content"
      class="yaml-editor"
      :auto-size="{ minRows: 24, maxRows: 40 }"
      placeholder="在这里编写流水线 YAML 配置"
      spellcheck="false"
    />

    <div class="editor-footer">
      <a-space>
        <a-button type="primary" :loading="saving" @click="save">保存</a-button>
        <a-button @click="goBack">取消</a-button>
        <a-button v-if="!isEdit" @click="fillTemplate">填入示例模板</a-button>
      </a-space>
      <span class="hint">配置以 YAML 格式保存到服务端本地文件系统</span>
    </div>
  </a-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { createPipeline, errorMessage, getPipelineRaw, updatePipeline } from '../api'

const route = useRoute()
const router = useRouter()

const name = computed(() => String(route.params.name ?? ''))
const isEdit = computed(() => route.name === 'pipeline-edit')

const content = ref('')
const error = ref('')
const saving = ref(false)

const TEMPLATE = `apiVersion: v1
name: my-first-pipeline
description: 示例流水线：拉取代码并编译
git:
  url: https://github.com/golang/example.git
  branch: master
workdir: .
env:
  CGO_ENABLED: "0"
stages:
  - name: build
    commands:
      - go version
      - go build ./...
#  - name: image
#    image: demo/my-first-pipeline:latest
#    dockerfile: Dockerfile
#    context: .
#    push: false
`

function fillTemplate() {
  content.value = TEMPLATE
}

async function loadRaw() {
  try {
    content.value = await getPipelineRaw(name.value)
  } catch (e) {
    error.value = '加载配置失败: ' + errorMessage(e)
  }
}

async function save() {
  error.value = ''
  if (!content.value.trim()) {
    error.value = '配置内容不能为空'
    return
  }
  saving.value = true
  try {
    const def = isEdit.value
      ? await updatePipeline(name.value, content.value)
      : await createPipeline(content.value)
    Message.success('保存成功')
    router.push(`/pipelines/${def.name}`)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

function goBack() {
  if (window.history.length > 1) router.back()
  else router.push('/pipelines')
}

onMounted(() => {
  if (isEdit.value) loadRaw()
})
</script>

<style scoped>
.yaml-editor :deep(textarea) {
  font-family: 'JetBrains Mono', 'Cascadia Code', Consolas, Monaco, monospace;
  font-size: 13px;
  line-height: 1.6;
  background: #fbfbfc;
}

.editor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
}

.hint {
  color: var(--color-text-3);
  font-size: 12px;
}
</style>
