import { ElMessage } from 'element-plus';

import type { ApiResponse } from '@/types/api';
import { clearAuth, getStoredToken } from '@/utils/authStorage';

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1';

// 业务层统一捕获这个错误类型，便于展示后端返回的错误信息。
export class ApiError extends Error {
  code: number;

  constructor(code: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
  }
}

// fetch 的统一封装：负责拼接基础地址、附加 token、解析统一响应格式。
export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const token = getStoredToken();

  // FormData 需要浏览器自动生成 multipart boundary，不能手动设置 Content-Type。
  if (!(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  // 后端通过 Bearer Token 鉴权，登录后每个受保护接口都需要这个请求头。
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
    // token 失效时主动清理本地状态，避免用户停留在一个必然失败的页面。
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

// 页面层调用这个 helper 后，不需要重复判断错误类型。
export function showApiError(error: unknown, fallback = '操作失败') {
  if (error instanceof ApiError) {
    ElMessage.error(error.message);
    return;
  }
  ElMessage.error(fallback);
}
