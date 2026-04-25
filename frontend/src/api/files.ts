import { request } from './request';
import type { DownloadResponse, FileListResponse, FileUploadResponse } from '@/types/api';

export interface ListFilesParams {
  folder_id: number;
  page: number;
  page_size: number;
}

export function listFiles(params: ListFilesParams) {
  const search = new URLSearchParams({
    folder_id: String(params.folder_id),
    page: String(params.page),
    page_size: String(params.page_size),
  });

  return request<FileListResponse>(`/files?${search.toString()}`);
}

export function uploadFile(file: File, parentId: number) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('parent_id', String(parentId));

  return request<FileUploadResponse>('/files/upload', {
    method: 'POST',
    body: formData,
  });
}

export function getDownloadUrl(id: number) {
  return request<DownloadResponse>(`/files/${id}/download`);
}

export function deleteFile(id: number) {
  return request<null>(`/files/${id}`, {
    method: 'DELETE',
  });
}

export function renameFile(id: number, name: string) {
  return request<null>(`/files/${id}/rename`, {
    method: 'PUT',
    body: JSON.stringify({ name }),
  });
}
