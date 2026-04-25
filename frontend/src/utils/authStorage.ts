import type { User } from '@/types/api';

const tokenKey = 'cloud-storage-token';
const expiresAtKey = 'cloud-storage-expires-at';
const userKey = 'cloud-storage-user';

export function getStoredToken() {
  return localStorage.getItem(tokenKey) || '';
}

export function getStoredExpiresAt() {
  return Number(localStorage.getItem(expiresAtKey)) || null;
}

export function getStoredUser() {
  const raw = localStorage.getItem(userKey);
  if (!raw) {
    return null;
  }

  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export function persistAuth(token: string, expiresAt: number | null, user: User | null) {
  if (token) {
    localStorage.setItem(tokenKey, token);
  } else {
    localStorage.removeItem(tokenKey);
  }

  if (expiresAt) {
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

export function clearAuth() {
  persistAuth('', null, null);
}
