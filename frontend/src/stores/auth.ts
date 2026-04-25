import { defineStore } from 'pinia';

import { getProfile, login, register } from '@/api/auth';
import type { LoginRequest, RegisterRequest, User } from '@/types/api';
import { getStoredExpiresAt, getStoredToken, getStoredUser, persistAuth } from '@/utils/authStorage';

interface AuthState {
  token: string;
  expiresAt: number | null;
  user: User | null;
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: getStoredToken(),
    expiresAt: getStoredExpiresAt(),
    user: getStoredUser(),
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.token),
  },
  actions: {
    persist() {
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
      await register(payload);
    },
    async refreshProfile() {
      if (!this.token) {
        return;
      }
      this.user = await getProfile();
      this.persist();
    },
    logout() {
      this.token = '';
      this.expiresAt = null;
      this.user = null;
      this.persist();
    },
  },
});
