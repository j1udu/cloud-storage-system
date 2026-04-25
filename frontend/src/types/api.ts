export interface ApiResponse<T> {
  code: number;
  msg: string;
  data: T;
}

export interface User {
  id: number;
  username: string;
  nickname: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface RegisterRequest {
  username: string;
  password: string;
  nickname?: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  expires_at: number;
  user: User;
}

export interface Matter {
  id: number;
  user_id: number;
  parent_id: number;
  name: string;
  dir: boolean;
  size: number;
  ext: string;
  mime_type: string;
  md5?: string;
  path?: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface FileListResponse {
  total: number;
  items: Matter[];
}

export interface FileUploadResponse {
  id: number;
  name: string;
  size: number;
  ext: string;
}

export interface DownloadResponse {
  url: string;
}
