<script setup lang="ts">
import { Collection, Delete, Document, Download, FolderOpened, Picture, Refresh, Share, SwitchButton, UploadFilled, FolderAdd, Rank, VideoCamera } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { computed, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

import {
  cancelShare, createFolder, createShare, deleteFile, getDownloadUrl, getFolderPath,
  getStorageQuota, listFiles, listRecycle, listShares, moveFile, permanentDeleteRecycleItem,
  renameFile, restoreRecycleItem, uploadFile,
} from '@/api/files';
import { showApiError } from '@/api/request';
import { useAuthStore } from '@/stores/auth';
import type { Matter, Share as ShareType, StorageQuota as QuotaType } from '@/types/api';
import { formatBytes, formatDate } from '@/utils/format';

interface FolderCrumb {
  id: number;
  name: string;
}

const router = useRouter();
const authStore = useAuthStore();
const origin = window.location.origin;

const loading = ref(false);
const uploadInputRef = ref<HTMLInputElement>();
const uploadLoading = ref(false);
const renameVisible = ref(false);
const renaming = ref(false);
const createFolderVisible = ref(false);
const creatingFolder = ref(false);
const currentFile = ref<Matter | null>(null);
const crumbs = ref<FolderCrumb[]>([{ id: 0, name: '根目录' }]);
const files = ref<Matter[]>([]);
const query = reactive({
  page: 1,
  pageSize: 8,
  total: 0,
});
const renameForm = reactive({ name: '' });
const createFolderForm = reactive({ name: '' });

// ========== 容量 ==========
const quota = ref<QuotaType | null>(null);

async function fetchQuota() {
  try {
    quota.value = await getStorageQuota();
  } catch { /* ignore */ }
}

const quotaPercent = computed(() => {
  if (!quota.value || quota.value.quota_bytes === 0) return 0;
  return Math.min((quota.value.used_bytes / quota.value.quota_bytes) * 100, 100);
});

const quotaDashOffset = computed(() => {
  const circumference = 2 * Math.PI * 54;
  return circumference - (quotaPercent.value / 100) * circumference;
});

// ========== 多选 ==========
const selectedIds = ref<Set<number>>(new Set());

function toggleSelect(row: Matter, event?: MouseEvent) {
  if (event) event.stopPropagation();
  if (selectedIds.value.has(row.id)) {
    selectedIds.value.delete(row.id);
  } else {
    selectedIds.value.add(row.id);
  }
}

function selectAll() {
  if (selectedIds.value.size === files.value.length) {
    selectedIds.value.clear();
  } else {
    selectedIds.value = new Set(files.value.map(f => f.id));
  }
}

function clearSelection() {
  selectedIds.value.clear();
}

const hasSelection = computed(() => selectedIds.value.size > 0);

// ========== 移动 ==========
const moveVisible = ref(false);
const moveLoading = ref(false);
const moveTargetId = ref(0);
const moveCrumbs = ref<FolderCrumb[]>([{ id: 0, name: '根目录' }]);
const moveFolders = ref<Matter[]>([]);
const movePendingIds = ref<number[]>([]);

function openMoveSingle(row: Matter) {
  movePendingIds.value = [row.id];
  moveTargetId.value = 0;
  moveCrumbs.value = [{ id: 0, name: '根目录' }];
  moveVisible.value = true;
  fetchMoveFolders(0);
}

function openMoveSelected() {
  if (selectedIds.value.size === 0) return;
  movePendingIds.value = [...selectedIds.value];
  moveTargetId.value = 0;
  moveCrumbs.value = [{ id: 0, name: '根目录' }];
  moveVisible.value = true;
  fetchMoveFolders(0);
}

async function fetchMoveFolders(folderId: number) {
  try {
    const data = await listFiles({ folder_id: folderId, page: 1, page_size: 200 });
    moveFolders.value = (data.items || []).filter(f => f.dir);
  } catch (error) {
    showApiError(error, '获取文件夹列表失败');
  }
}

async function enterMoveFolder(folder: Matter) {
  moveCrumbs.value.push({ id: folder.id, name: folder.name });
  moveTargetId.value = folder.id;
  await fetchMoveFolders(folder.id);
}

function jumpToMoveCrumb(index: number) {
  moveCrumbs.value = moveCrumbs.value.slice(0, index + 1);
  moveTargetId.value = moveCrumbs.value[moveCrumbs.value.length - 1].id;
  fetchMoveFolders(moveTargetId.value);
}

async function submitMove() {
  moveLoading.value = true;
  const failedItems: string[] = [];
  for (const id of movePendingIds.value) {
    const file = files.value.find(f => f.id === id);
    const name = file?.name || `ID:${id}`;
    try {
      await moveFile(id, moveTargetId.value);
    } catch (error) {
      const reason = error instanceof Error ? error.message : '';
      failedItems.push(`${name}（${reason || '失败'}）`);
    }
  }
  moveLoading.value = false;
  moveVisible.value = false;
  if (failedItems.length === 0) {
    ElMessage.success('移动成功');
  } else {
    ElMessage.warning(`以下文件移动失败：${failedItems.join('、')}`);
  }
  selectedIds.value.clear();
  await fetchFiles();
}

// ========== 拖放 ==========
const dragIds = ref<number[]>([]);
const dragOverId = ref<number | null>(null);
const dragOverRecycle = ref(false);

function onDragStart(row: Matter, event: DragEvent) {
  if (selectedIds.value.has(row.id)) {
    dragIds.value = [...selectedIds.value];
  } else {
    dragIds.value = [row.id];
    selectedIds.value.clear();
    selectedIds.value.add(row.id);
  }
  event.dataTransfer!.effectAllowed = 'move';
  event.dataTransfer!.setData('text/plain', JSON.stringify(dragIds.value));
}

function onDragOver(row: Matter, event: DragEvent) {
  if (!row.dir) return;
  event.preventDefault();
  event.dataTransfer!.dropEffect = 'move';
  dragOverId.value = row.id;
}

function onDragLeave() {
  dragOverId.value = null;
}

async function onDrop(target: Matter, event: DragEvent) {
  event.preventDefault();
  dragOverId.value = null;
  if (!target.dir) return;

  let ids: number[] = [];
  try { ids = JSON.parse(event.dataTransfer!.getData('text/plain')); } catch { return; }
  if (ids.includes(target.id)) return;

  const failedItems: string[] = [];
  for (const id of ids) {
    const file = files.value.find(f => f.id === id);
    const name = file?.name || `ID:${id}`;
    try { await moveFile(id, target.id); } catch (error) {
      const reason = error instanceof Error ? error.message : '';
      failedItems.push(`${name}（${reason || '失败'}）`);
    }
  }
  if (failedItems.length === 0) {
    ElMessage.success(`已移动到"${target.name}"`);
  } else {
    ElMessage.warning(`以下文件移动失败：${failedItems.join('、')}`);
  }
  selectedIds.value.clear();
  dragIds.value = [];
  await fetchFiles();
}

// 拖放到回收站
function onRecycleDragOver(event: DragEvent) {
  event.preventDefault();
  event.dataTransfer!.dropEffect = 'move';
  dragOverRecycle.value = true;
}

function onRecycleDragLeave() {
  dragOverRecycle.value = false;
}

async function onRecycleDrop(event: DragEvent) {
  event.preventDefault();
  dragOverRecycle.value = false;

  let ids: number[] = [];
  try { ids = JSON.parse(event.dataTransfer!.getData('text/plain')); } catch { return; }

  try {
    await ElMessageBox.confirm(`确定将 ${ids.length} 个项目移到回收站吗？`, '移到回收站', {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    });
  } catch { return; }

  const failedItems: string[] = [];
  for (const id of ids) {
    const file = files.value.find(f => f.id === id);
    const name = file?.name || `ID:${id}`;
    try { await deleteFile(id); } catch {
      failedItems.push(name);
    }
  }
  if (failedItems.length === 0) {
    ElMessage.success('已移到回收站');
  } else {
    ElMessage.warning(`${failedItems.join('、')} 删除失败`);
  }
  selectedIds.value.clear();
  dragIds.value = [];
  await fetchFiles();
  await fetchQuota();
}

// ========== 原有逻辑 ==========
const currentFolderId = computed(() => crumbs.value[crumbs.value.length - 1]?.id || 0);

async function fetchFiles() {
  loading.value = true;
  try {
    const data = await listFiles({
      folder_id: currentFolderId.value,
      page: query.page,
      page_size: query.pageSize,
    });
    files.value = data.items || [];
    query.total = data.total || 0;
  } catch (error) {
    showApiError(error, '获取文件列表失败');
  } finally {
    loading.value = false;
  }
}

async function openFolder(row: Matter) {
  if (!row.dir) return;
  crumbs.value.push({ id: row.id, name: row.name });
  query.page = 1;
  await fetchFiles();
  await updateBreadcrumbs();
}

async function jumpToCrumb(index: number) {
  crumbs.value = crumbs.value.slice(0, index + 1);
  query.page = 1;
  await fetchFiles();
  await updateBreadcrumbs();
}

async function updateBreadcrumbs() {
  try {
    const pathItems = await getFolderPath(currentFolderId.value);
    if (pathItems.length > 0) {
      crumbs.value = pathItems.map(item => ({ id: item.id, name: item.name }));
    } else {
      crumbs.value = [{ id: 0, name: '根目录' }];
    }
  } catch (error) {
    showApiError(error, '获取路径失败');
  }
}

function openCreateFolder() {
  createFolderForm.name = '';
  createFolderVisible.value = true;
}

async function submitCreateFolder() {
  if (!createFolderForm.name.trim()) {
    ElMessage.warning('请输入文件夹名称');
    return;
  }
  creatingFolder.value = true;
  try {
    await createFolder(currentFolderId.value, createFolderForm.name.trim());
    ElMessage.success('文件夹创建成功');
    createFolderVisible.value = false;
    await fetchFiles();
  } catch (error) {
    showApiError(error, '创建文件夹失败');
  } finally {
    creatingFolder.value = false;
  }
}

const MAX_UPLOAD_COUNT = 20;

function pickFile() {
  uploadInputRef.value?.click();
}

async function handleUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const toUpload = input.files ? Array.from(input.files) : [];
  input.value = '';
  if (toUpload.length === 0) return;

  if (toUpload.length > MAX_UPLOAD_COUNT) {
    ElMessage.warning(`单次最多上传 ${MAX_UPLOAD_COUNT} 个文件`);
    return;
  }

  const existingNames = new Set(files.value.map(f => f.name));
  const duplicates = toUpload.filter(f => existingNames.has(f.name));
  const unique = toUpload.filter(f => !existingNames.has(f.name));

  let replaceFiles: File[] = [];
  if (duplicates.length > 0) {
    try {
      await ElMessageBox.confirm(
        `以下文件与当前目录存在重名：\n${duplicates.map(f => f.name).join('\n')}\n\n点击"替换"覆盖已有文件，点击"跳过"保留已有文件`,
        '发现重名文件',
        { confirmButtonText: '替换', cancelButtonText: '跳过', distinguishCancelAndClose: true, type: 'warning' },
      );
      replaceFiles = duplicates;
    } catch (action) {
      if (action === 'cancel') { replaceFiles = []; } else { return; }
    }
  }

  const finalList = [...unique, ...replaceFiles];
  if (finalList.length === 0) { ElMessage.info('没有需要上传的文件'); return; }

  uploadLoading.value = true;
  let successCount = 0;
  const failedNames: string[] = [];

  for (const file of finalList) {
    if (replaceFiles.includes(file)) {
      const existing = files.value.find(f => f.name === file.name);
      if (existing) {
        try { await deleteFile(existing.id); } catch {
          failedNames.push(`${file.name}（替换旧文件失败）`);
          continue;
        }
      }
    }
    try { await uploadFile(file, currentFolderId.value); successCount++; } catch { failedNames.push(file.name); }
  }

  uploadLoading.value = false;
  if (failedNames.length === 0) {
    ElMessage.success(`${successCount} 个文件上传成功`);
  } else {
    ElMessage.warning(`${successCount} 个上传成功，${failedNames.join('、')} 上传失败`);
  }
  await fetchFiles();
  await fetchQuota();
}

async function download(row: Matter) {
  try {
    const data = await getDownloadUrl(row.id);
    window.open(data.url, '_blank', 'noopener,noreferrer');
  } catch (error) {
    showApiError(error, '获取下载链接失败');
  }
}

function openRename(row: Matter) {
  currentFile.value = row;
  renameForm.name = row.name;
  renameVisible.value = true;
}

async function submitRename() {
  if (!currentFile.value || !renameForm.name.trim()) {
    ElMessage.warning('请输入新名称');
    return;
  }
  renaming.value = true;
  try {
    await renameFile(currentFile.value.id, renameForm.name.trim());
    ElMessage.success('重命名成功');
    renameVisible.value = false;
    await fetchFiles();
  } catch (error) {
    showApiError(error, '重命名失败');
  } finally {
    renaming.value = false;
  }
}

async function remove(row: Matter) {
  try {
    await ElMessageBox.confirm(`确定删除"${row.name}"吗？`, '删除文件', {
      type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
    });
    await deleteFile(row.id);
    ElMessage.success('删除成功');
    await fetchFiles();
    await fetchQuota();
  } catch (error) {
    if (error !== 'cancel') showApiError(error, '删除失败');
  }
}

async function batchDownload() {
  const selected = files.value.filter(f => selectedIds.value.has(f.id) && !f.dir);
  if (selected.length === 0) { ElMessage.warning('没有可下载的文件（文件夹不支持下载）'); return; }
  const failedNames: string[] = [];
  for (const f of selected) {
    try {
      const data = await getDownloadUrl(f.id);
      const a = document.createElement('a');
      a.href = data.url; a.download = f.name; a.target = '_blank'; a.rel = 'noopener,noreferrer';
      document.body.appendChild(a); a.click(); document.body.removeChild(a);
      await new Promise(r => setTimeout(r, 300));
    } catch { failedNames.push(f.name); }
  }
  if (failedNames.length > 0) ElMessage.warning(`${failedNames.join('、')} 下载失败`);
}

async function batchDelete() {
  const ids = [...selectedIds.value];
  if (ids.length === 0) return;
  try {
    await ElMessageBox.confirm(`确定删除选中的 ${ids.length} 个项目吗？`, '批量删除', {
      type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
    });
    const failedNames: string[] = [];
    for (const id of ids) {
      const file = files.value.find(f => f.id === id);
      try { await deleteFile(id); } catch { if (file) failedNames.push(file.name); }
    }
    if (failedNames.length === 0) ElMessage.success('删除成功');
    else ElMessage.warning(`${failedNames.join('、')} 删除失败`);
    selectedIds.value.clear();
    await fetchFiles();
    await fetchQuota();
  } catch (error) {
    if (error !== 'cancel') showApiError(error, '删除失败');
  }
}

// ========== 回收站 ==========
const recycleVisible = ref(false);
const recycleLoading = ref(false);
const recycleFiles = ref<Matter[]>([]);
const recycleQuery = reactive({ page: 1, pageSize: 20, total: 0 });

async function openRecycle() {
  recycleVisible.value = true;
  recycleQuery.page = 1;
  await fetchRecycle();
}

async function fetchRecycle() {
  recycleLoading.value = true;
  try {
    const data = await listRecycle(recycleQuery.page, recycleQuery.pageSize);
    recycleFiles.value = data.items || [];
    recycleQuery.total = data.total || 0;
  } catch (error) {
    showApiError(error, '获取回收站列表失败');
  } finally {
    recycleLoading.value = false;
  }
}

async function restoreItem(row: Matter) {
  try {
    await restoreRecycleItem(row.id);
    ElMessage.success(`"${row.name}" 已恢复`);
    await fetchRecycle();
    await fetchFiles();
    await fetchQuota();
  } catch (error) {
    showApiError(error, '恢复失败');
  }
}

async function permanentDelete(row: Matter) {
  try {
    await ElMessageBox.confirm(`彻底删除"${row.name}"？此操作不可恢复`, '彻底删除', {
      type: 'warning', confirmButtonText: '彻底删除', cancelButtonText: '取消',
    });
    await permanentDeleteRecycleItem(row.id);
    ElMessage.success('已彻底删除');
    await fetchRecycle();
    await fetchQuota();
  } catch (error) {
    if (error !== 'cancel') showApiError(error, '彻底删除失败');
  }
}

function handleRecyclePageChange(page: number) {
  recycleQuery.page = page;
  void fetchRecycle();
}

// ========== 分享 ==========
const shareVisible = ref(false);
const shareLoading = ref(false);
const shareList = ref<ShareType[]>([]);
const shareQuery = reactive({ page: 1, pageSize: 20, total: 0 });

// 创建分享
const createShareVisible = ref(false);
const createShareLoading = ref(false);
const shareForm = reactive({ matter_id: 0, access_code: '', expire_hour: 0 });
const shareResult = ref<ShareType | null>(null);
const batchShareResults = ref<{ name: string; token: string }[]>([]);
const isBatchShare = ref(false);

function openCreateShare(row: Matter) {
  if (row.dir) {
    ElMessage.warning('暂不支持分享文件夹');
    return;
  }
  shareForm.matter_id = row.id;
  shareForm.access_code = '';
  shareForm.expire_hour = 0;
  shareResult.value = null;
  batchShareResults.value = [];
  isBatchShare.value = false;
  createShareVisible.value = true;
}

async function openBatchShare() {
  if (selectedIds.value.size === 0) return;
  const selectedFiles = files.value.filter(f => selectedIds.value.has(f.id));
  const hasFolder = selectedFiles.some(f => f.dir);
  if (hasFolder) {
    ElMessage.warning('暂不支持分享文件夹，请取消选中文件夹后重试');
    return;
  }
  shareForm.access_code = '';
  shareForm.expire_hour = 0;
  shareResult.value = null;
  batchShareResults.value = [];
  isBatchShare.value = true;
  createShareVisible.value = true;
}

async function submitCreateShare() {
  createShareLoading.value = true;
  try {
    if (isBatchShare.value) {
      const ids = [...selectedIds.value];
      for (const id of ids) {
        const file = files.value.find(f => f.id === id);
        const res = await createShare({
          matter_id: id,
          access_code: shareForm.access_code || undefined,
          expire_hour: shareForm.expire_hour || undefined,
        });
        batchShareResults.value.push({ name: file?.name || `ID:${id}`, token: res.token });
      }
      ElMessage.success(`已创建 ${batchShareResults.value.length} 个分享链接`);
    } else {
      shareResult.value = await createShare({
        matter_id: shareForm.matter_id,
        access_code: shareForm.access_code || undefined,
        expire_hour: shareForm.expire_hour || undefined,
      });
      ElMessage.success('分享链接已创建');
    }
  } catch (error) {
    showApiError(error, '创建分享失败');
  } finally {
    createShareLoading.value = false;
  }
}

function copyShareLink() {
  if (!shareResult.value) return;
  const link = `${origin}/share/${shareResult.value.token}`;
  navigator.clipboard.writeText(link).then(() => ElMessage.success('链接已复制')).catch(() => ElMessage.error('复制失败'));
}

function copyShareTokenLink(token: string) {
  const link = `${origin}/share/${token}`;
  navigator.clipboard.writeText(link).then(() => ElMessage.success('链接已复制')).catch(() => ElMessage.error('复制失败'));
}

async function openShareList() {
  shareVisible.value = true;
  shareQuery.page = 1;
  await fetchShares();
}

async function fetchShares() {
  shareLoading.value = true;
  try {
    const data = await listShares(shareQuery.page, shareQuery.pageSize);
    shareList.value = data.items || [];
    shareQuery.total = data.total || 0;
  } catch (error) {
    showApiError(error, '获取分享列表失败');
  } finally {
    shareLoading.value = false;
  }
}

async function cancelShareItem(share: ShareType) {
  try {
    await ElMessageBox.confirm('确定取消此分享？', '取消分享', {
      type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消',
    });
    await cancelShare(share.id);
    ElMessage.success('分享已取消');
    await fetchShares();
  } catch (error) {
    if (error !== 'cancel') showApiError(error, '取消分享失败');
  }
}

function handleSharePageChange(page: number) {
  shareQuery.page = page;
  void fetchShares();
}

// ========== 通用 ==========
async function logout() {
  authStore.logout();
  await router.replace('/login');
}

function handlePageChange(page: number) {
  query.page = page;
  void fetchFiles();
}

const fileTypeIconMap: Record<string, typeof Document> = {
  zip: Collection, rar: Collection, '7z': Collection, tar: Collection, gz: Collection,
  jpg: Picture, jpeg: Picture, png: Picture, gif: Picture, svg: Picture, webp: Picture, bmp: Picture,
  mp4: VideoCamera, avi: VideoCamera, mkv: VideoCamera, mov: VideoCamera, wmv: VideoCamera, flv: VideoCamera,
};

function getFileIcon(row: Matter) {
  if (row.dir) return FolderOpened;
  const ext = (row.ext || '').toLowerCase().replace(/^\./, '');
  return fileTypeIconMap[ext] || Document;
}

onMounted(async () => {
  await authStore.refreshProfile().catch(() => undefined);
  await Promise.all([fetchFiles(), fetchQuota()]);
});
</script>

<template>
  <main class="files-page">
    <header class="topbar">
      <div>
        <h1>个人云盘</h1>
        <p>{{ authStore.user?.nickname || authStore.user?.username || '已登录用户' }}</p>
      </div>
      <el-button :icon="SwitchButton" @click="logout">退出登录</el-button>
    </header>

    <div class="main-content">
      <aside class="dashboard-panel">
        <!-- 容量圆环 -->
        <div class="dashboard-inner">
          <svg class="quota-ring" viewBox="0 0 120 120">
            <circle class="quota-ring-bg" cx="60" cy="60" r="54" />
            <circle
              class="quota-ring-fill"
              cx="60" cy="60" r="54"
              :stroke-dashoffset="quotaDashOffset"
            />
          </svg>
          <div class="quota-ring-text">
            <span class="quota-used">{{ quota ? formatBytes(quota.used_bytes) : '--' }}</span>
            <span class="quota-total">{{ quota ? `/ ${formatBytes(quota.quota_bytes)}` : '' }}</span>
          </div>
          <h3>存储空间</h3>
          <p class="dashboard-hint">已使用 {{ quotaPercent.toFixed(1) }}%</p>
          <div class="dashboard-stats">
            <div class="stat-item">
              <span class="stat-value">{{ query.total }}</span>
              <span class="stat-label">当前目录项数</span>
            </div>
          </div>
        </div>

        <!-- 回收站卡片 -->
        <div
          :class="['recycle-card', { 'is-drag-over': dragOverRecycle }]"
          @click="openRecycle"
          @dragover="onRecycleDragOver"
          @dragleave="onRecycleDragLeave"
          @drop="onRecycleDrop"
        >
          <el-icon class="recycle-icon"><Delete /></el-icon>
          <span>回收站</span>
        </div>

        <!-- 我的分享入口 -->
        <div class="share-entry-card" @click="openShareList">
          <el-icon><Share /></el-icon>
          <span>我的分享</span>
        </div>
      </aside>

      <section class="files-area">
        <div class="toolbar">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item v-for="(item, index) in crumbs" :key="item.id">
              <button class="crumb-button" type="button" @click="jumpToCrumb(index)">{{ item.name }}</button>
            </el-breadcrumb-item>
          </el-breadcrumb>

          <div class="toolbar-actions">
            <input ref="uploadInputRef" class="hidden-input" type="file" multiple @change="handleUpload" />
            <transition-group name="slide-btn">
              <el-button v-if="hasSelection" key="move" :icon="Rank" @click="openMoveSelected">移动 ({{ selectedIds.size }})</el-button>
              <el-button v-if="hasSelection" key="share" :icon="Share" @click="openBatchShare">分享</el-button>
              <el-button v-if="hasSelection" key="download" :icon="Download" @click="batchDownload">下载</el-button>
              <el-button v-if="hasSelection" key="delete" :icon="Delete" @click="batchDelete">删除</el-button>
              <el-button v-if="hasSelection" key="cancel" @click="clearSelection">取消选择</el-button>
            </transition-group>
            <el-button @click="selectAll">{{ selectedIds.size === files.length && files.length > 0 ? '取消全选' : '全选' }}</el-button>
            <el-button type="primary" :icon="UploadFilled" :loading="uploadLoading" @click="pickFile">上传文件</el-button>
            <el-button :icon="FolderAdd" @click="openCreateFolder">新建文件夹</el-button>
            <el-button :icon="Refresh" @click="fetchFiles">刷新</el-button>
          </div>
        </div>

        <div v-loading="loading" class="card-grid">
          <div
            v-for="row in files"
            :key="row.id"
            :class="['file-card', { 'is-dir': row.dir }, { 'is-selected': selectedIds.has(row.id) }, { 'is-drag-over': dragOverId === row.id }]"
            draggable="true"
            @dblclick="openFolder(row)"
            @dragstart="onDragStart(row, $event)"
            @dragover="onDragOver(row, $event)"
            @dragleave="onDragLeave"
            @drop="onDrop(row, $event)"
          >
            <div class="card-header">
              <label class="card-checkbox" @click.stop="toggleSelect(row)">
                <input type="checkbox" :checked="selectedIds.has(row.id)" />
              </label>
              <div class="card-icon">
                <el-icon><component :is="getFileIcon(row)" /></el-icon>
              </div>
            </div>
            <div class="card-body">
              <div class="card-name" :title="row.name">{{ row.name }}</div>
              <div class="card-meta">
                <span>{{ row.dir ? '文件夹' : row.ext || '文件' }}</span>
                <span>{{ row.dir ? '-' : formatBytes(row.size) }}</span>
                <span>{{ formatDate(row.updated_at) }}</span>
              </div>
            </div>
            <div class="card-actions" :class="{ 'is-hidden': selectedIds.has(row.id) }">
              <el-button link type="primary" size="small" @click.stop="openMoveSingle(row)">移动</el-button>
              <el-button link type="primary" size="small" @click.stop="openCreateShare(row)">分享</el-button>
              <el-button link type="primary" size="small" @click.stop="openRename(row)">重命名</el-button>
              <el-button v-if="!row.dir" link type="primary" size="small" :icon="Download" @click.stop="download(row)">下载</el-button>
              <el-button link type="danger" size="small" :icon="Delete" @click.stop="remove(row)">删除</el-button>
            </div>
          </div>

          <div v-if="!loading && files.length === 0" class="card-empty">当前目录暂无文件</div>
        </div>

        <div class="pagination-row">
          <el-pagination background layout="prev, pager, next" :current-page="query.page" :page-size="query.pageSize" :total="query.total" @current-change="handlePageChange" />
        </div>
      </section>
    </div>

    <!-- 重命名弹窗 -->
    <el-dialog v-model="renameVisible" title="重命名" width="420px">
      <el-input v-model.trim="renameForm.name" placeholder="请输入新名称" @keyup.enter="submitRename" />
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" :loading="renaming" @click="submitRename">保存</el-button>
      </template>
    </el-dialog>

    <!-- 新建文件夹弹窗 -->
    <el-dialog v-model="createFolderVisible" title="新建文件夹" width="420px">
      <el-input v-model.trim="createFolderForm.name" placeholder="请输入文件夹名称" @keyup.enter="submitCreateFolder" />
      <template #footer>
        <el-button @click="createFolderVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingFolder" @click="submitCreateFolder">保存</el-button>
      </template>
    </el-dialog>

    <!-- 移动文件弹窗 -->
    <el-dialog v-model="moveVisible" title="移动到文件夹" width="520px">
      <div class="move-dialog-body">
        <div class="move-breadcrumb">
          <button v-for="(item, index) in moveCrumbs" :key="item.id" class="move-crumb" type="button" @click="jumpToMoveCrumb(index)">{{ item.name }}</button>
        </div>
        <div class="move-folder-list">
          <div v-for="folder in moveFolders" :key="folder.id" :class="['move-folder-item', { 'is-current-target': moveTargetId === folder.id }]" @dblclick="enterMoveFolder(folder)" @click="moveTargetId = folder.id">
            <el-icon><FolderOpened /></el-icon>
            <span>{{ folder.name }}</span>
          </div>
          <div v-if="moveFolders.length === 0" class="move-folder-empty">此目录下没有子文件夹</div>
        </div>
        <div class="move-target-hint">目标：{{ moveCrumbs[moveCrumbs.length - 1]?.name || '根目录' }}</div>
      </div>
      <template #footer>
        <el-button @click="moveVisible = false">取消</el-button>
        <el-button type="primary" :loading="moveLoading" @click="submitMove">移动</el-button>
      </template>
    </el-dialog>

    <!-- 回收站弹窗 -->
    <el-dialog v-model="recycleVisible" title="回收站" width="640px">
      <div v-loading="recycleLoading">
        <div class="recycle-list">
          <div v-for="row in recycleFiles" :key="row.id" class="recycle-item">
            <div class="recycle-item-info">
              <el-icon><component :is="getFileIcon(row)" /></el-icon>
              <div>
                <div class="recycle-item-name">{{ row.name }}</div>
                <div class="recycle-item-meta">{{ row.dir ? '文件夹' : formatBytes(row.size) }} · {{ formatDate(row.updated_at) }}</div>
              </div>
            </div>
            <div class="recycle-item-actions">
              <el-button link type="primary" size="small" @click="restoreItem(row)">恢复</el-button>
              <el-button link type="danger" size="small" @click="permanentDelete(row)">彻底删除</el-button>
            </div>
          </div>
          <div v-if="!recycleLoading && recycleFiles.length === 0" class="card-empty">回收站为空</div>
        </div>
        <div v-if="recycleQuery.total > recycleQuery.pageSize" class="pagination-row">
          <el-pagination background layout="prev, pager, next" :current-page="recycleQuery.page" :page-size="recycleQuery.pageSize" :total="recycleQuery.total" @current-change="handleRecyclePageChange" />
        </div>
      </div>
    </el-dialog>

    <!-- 创建分享弹窗 -->
    <el-dialog v-model="createShareVisible" :title="isBatchShare ? '批量分享' : '创建分享'" width="460px">
      <div v-if="!shareResult && batchShareResults.length === 0" class="share-form">
        <el-form label-position="top">
          <el-form-item label="提取码（可选）">
            <el-input v-model="shareForm.access_code" placeholder="留空则无需提取码" maxlength="32" />
          </el-form-item>
          <el-form-item label="有效期">
            <el-select v-model="shareForm.expire_hour" style="width: 100%">
              <el-option :value="0" label="永久有效" />
              <el-option :value="1" label="1 小时" />
              <el-option :value="24" label="1 天" />
              <el-option :value="168" label="7 天" />
              <el-option :value="720" label="30 天" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>
      <!-- 单文件分享结果 -->
      <div v-else-if="shareResult" class="share-result">
        <div class="share-result-label">分享链接</div>
        <div class="share-result-link">{{ `${origin}/share/${shareResult.token}` }}</div>
        <div v-if="shareResult.access_code" class="share-result-code">提取码：{{ shareResult.access_code }}</div>
        <div v-if="shareResult.expire_at" class="share-result-expire">有效期至：{{ formatDate(shareResult.expire_at) }}</div>
        <el-button type="primary" @click="copyShareLink">复制链接</el-button>
      </div>
      <!-- 批量分享结果 -->
      <div v-else-if="batchShareResults.length > 0" class="share-result">
        <div class="share-result-label">已创建 {{ batchShareResults.length }} 个分享链接</div>
        <div class="batch-share-list">
          <div v-for="item in batchShareResults" :key="item.token" class="batch-share-item">
            <span class="batch-share-name">{{ item.name }}</span>
            <button class="batch-share-copy" @click="copyShareTokenLink(item.token)">复制</button>
          </div>
        </div>
      </div>
      <template #footer>
        <template v-if="!shareResult && batchShareResults.length === 0">
          <el-button @click="createShareVisible = false">取消</el-button>
          <el-button type="primary" :loading="createShareLoading" @click="submitCreateShare">创建</el-button>
        </template>
        <template v-else>
          <el-button @click="createShareVisible = false">关闭</el-button>
        </template>
      </template>
    </el-dialog>

    <!-- 我的分享列表弹窗 -->
    <el-dialog v-model="shareVisible" title="我的分享" width="640px">
      <div v-loading="shareLoading">
        <div class="share-list">
          <div v-for="item in shareList" :key="item.id" class="share-list-item">
            <div class="share-list-info">
              <div class="share-list-name">{{ item.matter_name || '未知文件' }}</div>
              <div class="share-list-meta">
                <span>{{ item.access_code ? `提取码: ${item.access_code}` : '无提取码' }}</span>
                <span>{{ item.status === 1 ? '有效' : '已取消' }}</span>
                <span>{{ formatDate(item.created_at) }}</span>
              </div>
            </div>
            <div class="share-list-actions">
              <el-button link type="primary" size="small" @click="copyShareTokenLink(item.token)">复制链接</el-button>
              <el-button v-if="item.status === 1" link type="danger" size="small" @click="cancelShareItem(item)">取消分享</el-button>
            </div>
          </div>
          <div v-if="!shareLoading && shareList.length === 0" class="card-empty">暂无分享</div>
        </div>
        <div v-if="shareQuery.total > shareQuery.pageSize" class="pagination-row">
          <el-pagination background layout="prev, pager, next" :current-page="shareQuery.page" :page-size="shareQuery.pageSize" :total="shareQuery.total" @current-change="handleSharePageChange" />
        </div>
      </div>
    </el-dialog>
  </main>
</template>
