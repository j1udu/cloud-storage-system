<script setup lang="ts">
import { Collection, Document, Download, FolderOpened, Picture, VideoCamera } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { getPublicShareInfo } from '@/api/files';
import type { PublicShareInfo } from '@/types/api';
import { formatBytes, formatDate } from '@/utils/format';

const fileTypeIconMap: Record<string, typeof Document> = {
  zip: Collection, rar: Collection, '7z': Collection, tar: Collection, gz: Collection,
  jpg: Picture, jpeg: Picture, png: Picture, gif: Picture, svg: Picture, webp: Picture, bmp: Picture,
  mp4: VideoCamera, avi: VideoCamera, mkv: VideoCamera, mov: VideoCamera, wmv: VideoCamera, flv: VideoCamera,
};

function getFileIcon(matter: { dir: boolean; ext?: string }) {
  if (matter.dir) return FolderOpened;
  const ext = (matter.ext || '').toLowerCase().replace(/^\./, '');
  return fileTypeIconMap[ext] || Document;
}

const route = useRoute();
const token = computed(() => route.params.token as string);

const loading = ref(true);
const error = ref('');
const shareInfo = ref<PublicShareInfo | null>(null);
const accessCode = ref('');
const codeError = ref('');
const downloading = ref(false);

async function fetchShareInfo() {
  loading.value = true;
  error.value = '';
  try {
    shareInfo.value = await getPublicShareInfo(token.value);
  } catch (e) {
    error.value = e instanceof Error ? e.message : '获取分享信息失败';
  } finally {
    loading.value = false;
  }
}

async function handleDownload() {
  if (!shareInfo.value) return;
  if (shareInfo.value.has_code && !accessCode.value.trim()) {
    codeError.value = '请输入提取码后再下载';
    return;
  }
  downloading.value = true;
  codeError.value = '';
  try {
    // 直接通过 API 流式下载文件，不再经过 MinIO 预签名 URL
    const resp = await fetch(`/api/v1/public/shares/${token.value}/download`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ access_code: accessCode.value }),
    });

    // 检查响应类型：如果是 JSON 说明后端返回了错误
    const contentType = resp.headers.get('content-type') || '';
    if (!contentType.includes('application/json')) {
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = shareInfo.value.matter.name;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      return;
    }

    // JSON 错误响应
    const errData = await resp.json();
    const msg = errData.msg || '下载失败';
    if (msg.includes('access_code')) {
      codeError.value = '提取码错误';
    } else {
      ElMessage.error(msg);
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : '下载失败';
    if (msg.includes('access_code')) {
      codeError.value = '提取码错误';
    } else {
      ElMessage.error(msg);
    }
  } finally {
    downloading.value = false;
  }
}

onMounted(() => {
  fetchShareInfo();
});
</script>

<template>
  <main class="share-page">
    <div class="share-card">
      <!-- 加载中 -->
      <div v-if="loading" class="share-status">
        <div class="share-spinner"></div>
        <p>加载中...</p>
      </div>

      <!-- 错误 -->
      <div v-else-if="error" class="share-status">
        <span class="share-error-icon">✕</span>
        <h2>分享不存在或已失效</h2>
        <p>{{ error }}</p>
      </div>

      <!-- 分享内容 -->
      <div v-else-if="shareInfo" class="share-content">
        <div class="share-header">
          <h2>文件分享</h2>
          <p class="share-from">来自 {{ shareInfo.sharer_name || '未知用户' }} 的分享</p>
        </div>

        <div class="share-file-card">
          <div class="share-file-icon">
            <el-icon :size="40">
              <component :is="getFileIcon(shareInfo.matter)" />
            </el-icon>
          </div>
          <div class="share-file-info">
            <div class="share-file-name">{{ shareInfo.matter.name }}</div>
            <div class="share-file-meta">
              <span>{{ shareInfo.matter.dir ? '文件夹' : shareInfo.matter.ext || '文件' }}</span>
              <span>{{ shareInfo.matter.dir ? '' : formatBytes(shareInfo.matter.size) }}</span>
              <span>{{ formatDate(shareInfo.matter.created_at) }}</span>
            </div>
          </div>
        </div>

        <!-- 提取码输入（仅当 has_code 为 true 时显示） -->
        <div v-if="shareInfo.has_code" class="share-code-section">
          <label class="share-code-label">此分享需要提取码</label>
          <div class="share-code-form">
            <input
              v-model="accessCode"
              class="share-code-input"
              placeholder="请输入提取码"
              maxlength="32"
              @input="codeError = ''"
            />
          </div>
          <p v-if="codeError" class="share-error-msg">{{ codeError }}</p>
        </div>

        <div class="share-details">
          <div class="share-detail-row">
            <span class="share-detail-label">分享时间</span>
            <span class="share-detail-value">{{ formatDate(shareInfo.matter.created_at) }}</span>
          </div>
        </div>

        <div v-if="!shareInfo.matter.dir" class="share-actions">
          <button class="share-download-btn" :disabled="downloading" @click="handleDownload">
            <el-icon><Download /></el-icon>
            {{ downloading ? '获取链接中...' : '下载文件' }}
          </button>
        </div>
        <div v-else class="share-actions">
          <p class="share-folder-hint">文件夹分享暂不支持直接下载</p>
        </div>
      </div>
    </div>
  </main>
</template>
