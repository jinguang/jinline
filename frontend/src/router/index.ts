import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/pipelines' },
    {
      path: '/pipelines',
      name: 'pipelines',
      component: () => import('../views/PipelineList.vue'),
    },
    {
      path: '/pipelines/new',
      name: 'pipeline-new',
      component: () => import('../views/PipelineEdit.vue'),
    },
    {
      path: '/pipelines/:name',
      name: 'pipeline-detail',
      component: () => import('../views/PipelineDetail.vue'),
    },
    {
      path: '/pipelines/:name/edit',
      name: 'pipeline-edit',
      component: () => import('../views/PipelineEdit.vue'),
    },
    {
      path: '/runs',
      name: 'runs',
      component: () => import('../views/RunList.vue'),
    },
    {
      path: '/runs/:id',
      name: 'run-detail',
      component: () => import('../views/RunDetail.vue'),
    },
    { path: '/:pathMatch(.*)*', redirect: '/pipelines' },
  ],
})

export default router
