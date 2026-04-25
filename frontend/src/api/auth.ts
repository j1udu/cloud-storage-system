import { request } from './request';
import type { LoginRequest, LoginResponse, RegisterRequest, User } from '@/types/api';

export function login(data: LoginRequest) {
  return request<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function register(data: RegisterRequest) {
  return request<User>('/auth/register', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function getProfile() {
  return request<User>('/auth/profile');
}
