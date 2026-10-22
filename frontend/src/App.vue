<template>
  <a-config-provider :locale="zhCN">
    <a-layout class="app-layout">
      <a-layout-sider :width="208" theme="dark" class="app-sider">
        <div class="logo" @click="$router.push('/pipelines')">
          <span class="logo-mark">JI</span>
          <span class="logo-text">jinline</span>
        </div>
        <a-menu theme="dark" :selected-keys="[activeKey]" @menu-item-click="onMenu">
          <a-menu-item key="pipelines">
            <template #icon><icon-code /></template>
            流水线
          </a-menu-item>
          <a-menu-item key="runs">
            <template #icon><icon-history /></template>
            运行记录
          </a-menu-item>
        </a-menu>
        <div class="sider-footer">CI / CD Pipeline</div>
      </a-layout-sider>

      <a-layout-content class="app-content">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'

const route = useRoute()
const router = useRouter()

const activeKey = computed(() => (route.path.startsWith('/runs') ? 'runs' : 'pipelines'))

function onMenu(key: string) {
  if (key === 'pipelines') router.push('/pipelines')
  if (key === 'runs') router.push('/runs')
}
</script>

<style scoped>
.app-layout {
  height: 100vh;
}

.app-sider {
  display: flex;
  flex-direction: column;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 20px;
  color: #fff;
  cursor: pointer;
}

.logo-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: rgb(var(--primary-6));
  font-size: 13px;
  font-weight: 700;
}

.logo-text {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 0.5px;
}

.sider-footer {
  position: absolute;
  bottom: 16px;
  width: 100%;
  text-align: center;
  color: var(--color-text-3);
  font-size: 12px;
}

.app-content {
  padding: 16px;
  overflow: auto;
}
</style>
