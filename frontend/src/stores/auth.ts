import { defineStore } from 'pinia';

import { getProfile, login, register } from '@/api/auth';
import type { LoginRequest, RegisterRequest, User } from '@/types/api';
import { getStoredExpiresAt, getStoredToken, getStoredUser, persistAuth } from '@/utils/authStorage';

interface AuthState {
  token: string;
  expiresAt: number | null;
  user: User | null;
}

function getCurrentUnixTime() {
  return Math.floor(Date.now() / 1000);
}

// 认证状态集中放在 Pinia 中，页面和请求层都可以围绕它判断登录态。
export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    // 初始化时读取 localStorage，使刷新页面后仍保留登录信息。
    token: getStoredToken(),
    expiresAt: getStoredExpiresAt(),
    user: getStoredUser(),
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.token && state.expiresAt && state.expiresAt > getCurrentUnixTime()),
  },
  actions: {
    persist() {
      // 每次登录态变化后同步到 localStorage。
      persistAuth(this.token, this.expiresAt, this.user);
    },
    async login(payload: LoginRequest) {
      const data = await login(payload);
      this.token = data.token;
      this.expiresAt = data.expires_at;
      this.user = data.user;
      this.persist();
    },
    async register(payload: RegisterRequest) {
      // 注册成功后由页面决定是否跳转或切换到登录模式。
      await register(payload);
    },
    async refreshProfile() {
      if (!this.isAuthenticated) {
        this.logout();
        return;
      }
      // 用后端最新的用户信息覆盖本地缓存。
      this.user = await getProfile();
      this.persist();
    },
    clearExpiredAuth() {
      if (this.token && !this.isAuthenticated) {
        this.logout();
      }
    },
    logout() {
      this.token = '';
      this.expiresAt = null;
      this.user = null;
      this.persist();
    },
  },
});
