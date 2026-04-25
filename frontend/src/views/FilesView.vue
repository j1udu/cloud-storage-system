<script setup lang="ts">
import {
  Delete,
  Download,
  Folder,
  FolderOpened,
  Refresh,
  SwitchButton,
  UploadFilled,
} from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { computed, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

import { deleteFile, getDownloadUrl, listFiles, renameFile, uploadFile } from '@/api/files';
import { showApiError } from '@/api/request';
import { useAuthStore } from '@/stores/auth';
import type { Matter } from '@/types/api';
import { formatBytes, formatDate } from '@/utils/format';

interface FolderCrumb {
  id: number;
  name: string;
}

const router = useRouter();
const authStore = useAuthStore();

const loading = ref(false);
const uploadInputRef = ref<HTMLInputElement>();
const uploadLoading = ref(false);
const renameVisible = ref(false);
const renaming = ref(false);
const currentFile = ref<Matter | null>(null);
const crumbs = ref<FolderCrumb[]>([{ id: 0, name: '全部文件' }]);
const files = ref<Matter[]>([]);
const query = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
});
const renameForm = reactive({
  name: '',
});

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

function openFolder(row: Matter) {
  if (!row.dir) {
    return;
  }
  crumbs.value.push({ id: row.id, name: row.name });
  query.page = 1;
  void fetchFiles();
}

function jumpToCrumb(index: number) {
  crumbs.value = crumbs.value.slice(0, index + 1);
  query.page = 1;
  void fetchFiles();
}

function pickFile() {
  uploadInputRef.value?.click();
}

async function handleUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';

  if (!file) {
    return;
  }

  uploadLoading.value = true;
  try {
    await uploadFile(file, currentFolderId.value);
    ElMessage.success('上传成功');
    await fetchFiles();
  } catch (error) {
    showApiError(error, '上传失败');
  } finally {
    uploadLoading.value = false;
  }
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
    await ElMessageBox.confirm(`确定删除“${row.name}”吗？`, '删除文件', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    });
    await deleteFile(row.id);
    ElMessage.success('删除成功');
    await fetchFiles();
  } catch (error) {
    if (error !== 'cancel') {
      showApiError(error, '删除失败');
    }
  }
}

async function logout() {
  authStore.logout();
  await router.replace('/login');
}

function handlePageChange(page: number) {
  query.page = page;
  void fetchFiles();
}

onMounted(async () => {
  await authStore.refreshProfile().catch(() => undefined);
  await fetchFiles();
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

    <section class="file-shell">
      <div class="toolbar">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item v-for="(item, index) in crumbs" :key="item.id">
            <button class="crumb-button" type="button" @click="jumpToCrumb(index)">
              {{ item.name }}
            </button>
          </el-breadcrumb-item>
        </el-breadcrumb>

        <div class="toolbar-actions">
          <input ref="uploadInputRef" class="hidden-input" type="file" @change="handleUpload" />
          <el-button type="primary" :icon="UploadFilled" :loading="uploadLoading" @click="pickFile">上传文件</el-button>
          <el-button :icon="Refresh" @click="fetchFiles">刷新</el-button>
        </div>
      </div>

      <el-table
        v-loading="loading"
        :data="files"
        class="file-table"
        empty-text="当前目录暂无文件"
        row-key="id"
        @row-dblclick="openFolder"
      >
        <el-table-column label="名称" min-width="260">
          <template #default="{ row }: { row: Matter }">
            <button :class="['file-name', { clickable: row.dir }]" type="button" @click="openFolder(row)">
              <el-icon>
                <FolderOpened v-if="row.dir" />
                <Folder v-else />
              </el-icon>
              <span>{{ row.name }}</span>
            </button>
          </template>
        </el-table-column>

        <el-table-column label="类型" width="120">
          <template #default="{ row }: { row: Matter }">
            {{ row.dir ? '文件夹' : row.ext || '文件' }}
          </template>
        </el-table-column>

        <el-table-column label="大小" width="120">
          <template #default="{ row }: { row: Matter }">
            {{ row.dir ? '-' : formatBytes(row.size) }}
          </template>
        </el-table-column>

        <el-table-column label="更新时间" width="180">
          <template #default="{ row }: { row: Matter }">
            {{ formatDate(row.updated_at) }}
          </template>
        </el-table-column>

        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }: { row: Matter }">
            <el-button link type="primary" @click="openRename(row)">重命名</el-button>
            <el-button v-if="!row.dir" link type="primary" :icon="Download" @click="download(row)">下载</el-button>
            <el-button link type="danger" :icon="Delete" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <el-pagination
          background
          layout="prev, pager, next, total"
          :current-page="query.page"
          :page-size="query.pageSize"
          :total="query.total"
          @current-change="handlePageChange"
        />
      </div>
    </section>

    <el-dialog v-model="renameVisible" title="重命名" width="420px">
      <el-input v-model.trim="renameForm.name" placeholder="请输入新名称" @keyup.enter="submitRename" />
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" :loading="renaming" @click="submitRename">保存</el-button>
      </template>
    </el-dialog>
  </main>
</template>
