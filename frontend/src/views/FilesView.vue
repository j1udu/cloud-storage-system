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

// 页面状态拆成列表加载、上传、重命名弹窗等几类，分别驱动不同 UI。
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

// 当前目录由面包屑的最后一个节点决定，根目录 id 约定为 0。
const currentFolderId = computed(() => crumbs.value[crumbs.value.length - 1]?.id || 0);

async function fetchFiles() {
  // 文件列表查询依赖当前目录和分页参数，成功后同步表格数据和总数。
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
  // 双击或点击文件夹时进入下一级；普通文件没有目录行为。
  if (!row.dir) {
    return;
  }
  crumbs.value.push({ id: row.id, name: row.name });
  query.page = 1;
  void fetchFiles();
}

function jumpToCrumb(index: number) {
  // 点击面包屑会截断路径，回到历史目录层级。
  crumbs.value = crumbs.value.slice(0, index + 1);
  query.page = 1;
  void fetchFiles();
}

function pickFile() {
  // 使用隐藏 input 保留浏览器原生文件选择能力，同时让按钮样式保持统一。
  uploadInputRef.value?.click();
}

async function handleUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  // 清空 input，保证连续选择同一个文件时也会触发 change。
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
    // 后端只返回预签名 URL，真正的文件下载由对象存储链接完成。
    const data = await getDownloadUrl(row.id);
    window.open(data.url, '_blank', 'noopener,noreferrer');
  } catch (error) {
    showApiError(error, '获取下载链接失败');
  }
}

function openRename(row: Matter) {
  // 打开弹窗前记录当前文件，并用原文件名初始化输入框。
  currentFile.value = row;
  renameForm.name = row.name;
  renameVisible.value = true;
}

async function submitRename() {
  // 前端先做最基本的空值保护，详细规则仍以后端校验为准。
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
    // 删除是高风险操作，先让用户确认，避免误触。
    await ElMessageBox.confirm(`确定删除“${row.name}”吗？`, '删除文件', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    });
    await deleteFile(row.id);
    ElMessage.success('删除成功');
    await fetchFiles();
  } catch (error) {
    // console.log('捕获到的错误：', error);
    if (error !== 'cancel') {
      showApiError(error, '删除失败');
    }
  }
}

async function logout() {
  // 退出时清空本地登录态，再回到登录页。
  authStore.logout();
  await router.replace('/login');
}

function handlePageChange(page: number) {
  // Element Plus 分页组件只负责发出页码变化，实际数据仍由 fetchFiles 拉取。
  query.page = page;
  void fetchFiles();
}

onMounted(async () => {
  // 进入文件页后尝试刷新用户信息，即使失败也继续拉取文件列表并交给接口层处理鉴权。
  await authStore.refreshProfile().catch(() => undefined);
  await fetchFiles();
});
</script>

<template>
  <main class="files-page">
    <!-- 顶栏展示当前用户信息，并提供退出入口。 -->
    <header class="topbar">
      <div>
        <h1>个人云盘</h1>
        <p>{{ authStore.user?.nickname || authStore.user?.username || '已登录用户' }}</p>
      </div>
      <el-button :icon="SwitchButton" @click="logout">退出登录</el-button>
    </header>

    <section class="file-shell">
      <div class="toolbar">
        <!-- 面包屑表示当前目录层级，点击任意层级可返回。 -->
        <el-breadcrumb separator="/">
          <el-breadcrumb-item v-for="(item, index) in crumbs" :key="item.id">
            <button class="crumb-button" type="button" @click="jumpToCrumb(index)">
              {{ item.name }}
            </button>
          </el-breadcrumb-item>
        </el-breadcrumb>

        <div class="toolbar-actions">
          <!-- 隐藏原生文件输入框，由上传按钮触发它。 -->
          <input ref="uploadInputRef" class="hidden-input" type="file" @change="handleUpload" />
          <el-button type="primary" :icon="UploadFilled" :loading="uploadLoading" @click="pickFile">上传文件</el-button>
          <el-button :icon="Refresh" @click="fetchFiles">刷新</el-button>
        </div>
      </div>

      <!-- 文件和文件夹共用同一张表，dir 字段决定图标和可操作项。 -->
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

      <!-- 分页状态由 query 管理，切页后重新请求后端。 -->
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

    <!-- 重命名弹窗复用 currentFile 和 renameForm 两个状态。 -->
    <el-dialog v-model="renameVisible" title="重命名" width="420px">
      <el-input v-model.trim="renameForm.name" placeholder="请输入新名称" @keyup.enter="submitRename" />
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" :loading="renaming" @click="submitRename">保存</el-button>
      </template>
    </el-dialog>
  </main>
</template>
