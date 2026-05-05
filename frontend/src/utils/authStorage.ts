import type { User } from '@/types/api';

// localStorage 的 key 集中定义，避免多个文件硬编码同一字符串。
const tokenKey = 'cloud-storage-token';
const expiresAtKey = 'cloud-storage-expires-at';
const userKey = 'cloud-storage-user';

export function getStoredToken() {
  return localStorage.getItem(tokenKey) || '';
}

export function getStoredExpiresAt() {
  const raw = localStorage.getItem(expiresAtKey);
  if (raw === null) {
    return null;
  }

  const value = Number(raw);
  return Number.isFinite(value) ? value : null;
}

export function getStoredUser() {
  const raw = localStorage.getItem(userKey);
  if (!raw) {
    return null;
  }

  try {
    // 用户对象以 JSON 字符串保存，读取失败时视为没有缓存。
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

// 根据当前登录态写入或清理 localStorage。
export function persistAuth(token: string, expiresAt: number | null, user: User | null) {
  if (token) {
    localStorage.setItem(tokenKey, token);
  } else {
    localStorage.removeItem(tokenKey);
  }

  if (expiresAt !== null && Number.isFinite(expiresAt)) {
    localStorage.setItem(expiresAtKey, String(expiresAt));
  } else {
    localStorage.removeItem(expiresAtKey);
  }

  if (user) {
    localStorage.setItem(userKey, JSON.stringify(user));
  } else {
    localStorage.removeItem(userKey);
  }
}

// 请求层发现 token 失效时会调用这里做统一清理。
export function clearAuth() {
  persistAuth('', null, null);
}
