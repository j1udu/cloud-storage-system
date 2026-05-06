import { request } from './request';
import type { DownloadResponse, FileListResponse, FileUploadResponse, Matter, PathItem } from '@/types/api';

export interface ListFilesParams {
  folder_id: number;
  page: number;
  page_size: number;
}

// 查询当前目录下的文件/文件夹，分页参数交给后端处理。
export function listFiles(params: ListFilesParams) {
  const search = new URLSearchParams({
    folder_id: String(params.folder_id),
    page: String(params.page),
    page_size: String(params.page_size),
  });

  return request<FileListResponse>(`/files?${search.toString()}`);
}

// 文件上传必须使用 FormData，字段名 file/parent_id 需要与后端处理器一致。
export function uploadFile(file: File, parentId: number) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('parent_id', String(parentId));

  return request<FileUploadResponse>('/files/upload', {
    method: 'POST',
    body: formData,
  });
}

// 后端返回的是对象存储的预签名 URL，前端拿到后再打开下载。
export function getDownloadUrl(id: number) {
  return request<DownloadResponse>(`/files/${id}/download`);
}

// 删除操作目前对应后端软删除，成功后页面会刷新列表。
export function deleteFile(id: number) {
  return request<null>(`/files/${id}`, {
    method: 'DELETE',
  });
}

// 重命名只需要提交新的 name，文件内容和存储对象不变。
export function renameFile(id: number, name: string) {
  return request<null>(`/files/${id}/rename`, {
    method: 'PUT',
    body: JSON.stringify({ name }),
  });
}

// 创建文件夹
export function createFolder(parentId: number, name: string) {
  return request<Matter>('/folders', {
    method: 'POST',
    body: JSON.stringify({ parent_id: parentId, name }),
  });
}

// 获取面包屑路径
export function getFolderPath(folderId: number) {
  return request<PathItem[]>(`/folders/path?folder_id=${folderId}`);
}
