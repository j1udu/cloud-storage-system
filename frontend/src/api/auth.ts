import { request } from './request';
import type { LoginRequest, LoginResponse, RegisterRequest, User } from '@/types/api';

// 登录成功后后端会返回 token 和用户信息。
export function login(data: LoginRequest) {
  return request<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// 注册接口只负责创建账号，当前页面逻辑会在成功后切回登录模式。
export function register(data: RegisterRequest) {
  return request<User>('/auth/register', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// 根据 token 获取当前用户资料，用于刷新页面后的用户信息恢复。
export function getProfile() {
  return request<User>('/auth/profile');
}
