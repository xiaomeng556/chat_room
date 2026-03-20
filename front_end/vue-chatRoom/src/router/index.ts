import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '../services/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/chat' },
    { path: '/login', component: () => import('../views/LoginView.vue') },
    { path: '/register', component: () => import('../views/RegisterView.vue') },
    { path: '/chat', component: () => import('../views/ChatView.vue'), meta: { requiresAuth: true } },
  ],
})

router.beforeEach((to) => {
  if (to.meta.requiresAuth && !getToken()) {
    return { path: '/login' }
  }
  if ((to.path === '/login' || to.path === '/register') && getToken()) {
    return { path: '/chat' }
  }
  return true
})

export default router

