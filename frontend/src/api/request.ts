import { ElMessage } from 'element-plus';

import type { ApiResponse } from '@/types/api';
import { clearAuth, getStoredToken } from '@/utils/authStorage';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1';

export class ApiError extends Error {
  code: number;

  constructor(code: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
  }
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const token = getStoredToken();

  if (!(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    throw new ApiError(response.status, `请求失败：${response.status}`);
  }

  const payload = (await response.json()) as ApiResponse<T>;
  if (payload.code !== 0) {
    if (payload.code === 10004 || payload.msg.includes('token') || payload.msg.includes('令牌')) {
      clearAuth();
      if (window.location.pathname !== '/login') {
        window.location.assign('/login');
      }
    }
    throw new ApiError(payload.code, payload.msg || '请求失败');
  }

  return payload.data;
}

export function showApiError(error: unknown, fallback = '操作失败') {
  if (error instanceof ApiError) {
    ElMessage.error(error.message);
    return;
  }
  ElMessage.error(fallback);
}
