// 后端所有 JSON API 的统一响应结构。
export interface ApiResponse<T> {
  code: number;
  msg: string;
  data: T;
}

// 当前登录用户信息。
export interface User {
  id: number;
  username: string;
  nickname: string;
  status: number;
  created_at: string;
  updated_at: string;
}

// 注册和登录请求体。
export interface RegisterRequest {
  username: string;
  password: string;
  nickname?: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

// 登录成功后用于恢复会话的数据。
export interface LoginResponse {
  token: string;
  expires_at: number;
  user: User;
}

// 文件系统条目；dir 为 true 时表示文件夹，为 false 时表示普通文件。
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

// 文件列表分页响应。
export interface FileListResponse {
  total: number;
  items: Matter[];
}

// 上传成功后的简要文件信息。
export interface FileUploadResponse {
  id: number;
  name: string;
  size: number;
  ext: string;
}

// 下载接口返回对象存储的预签名 URL。
export interface DownloadResponse {
  url: string;
}

// 创建文件夹请求体
export interface CreateFolderRequest {
  parent_id: number;
  name: string;
}

// 面包屑路径项
export interface PathItem {
  id: number;
  name: string;
}
