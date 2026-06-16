import { createRouter, createWebHashHistory } from 'vue-router'
import MediaLibrary from '../views/MediaLibrary.vue'
import VideoDetail from '../views/VideoDetail.vue'
import Tasks from '../views/Tasks.vue'

const routes = [
  { path: '/', redirect: '/media' },
  { path: '/media', component: MediaLibrary },
  { path: '/video/:hash', component: VideoDetail, props: true },
  { path: '/tasks', component: Tasks },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
