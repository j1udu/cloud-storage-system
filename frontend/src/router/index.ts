import { createRouter, createWebHistory } from 'vue-router';

import { useAuthStore } from '@/stores/auth';

// 路由表只保留两个业务页面：认证页和文件管理页。
const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/files',
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { guestOnly: true },
    },
    {
      path: '/files',
      name: 'files',
      component: () => import('@/views/FilesView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/share/:token',
      name: 'share',
      component: () => import('@/views/ShareView.vue'),
      meta: { public: true },
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/files',
    },
  ],
});

// 全局路由守卫：根据登录状态决定是否允许进入目标页面。
router.beforeEach((to) => {
  const authStore = useAuthStore();
  authStore.clearExpiredAuth();

  // 公开页面不需要任何检查
  if (to.meta.public) return true;

  // 需要登录的页面没有有效 token 时，跳回登录页并记录原始目标地址。
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } };
  }

  // 已登录用户访问登录页时，直接回到文件管理页。
  if (to.meta.guestOnly && authStore.isAuthenticated) {
    return { name: 'files' };
  }

  return true;
});

export default router;
