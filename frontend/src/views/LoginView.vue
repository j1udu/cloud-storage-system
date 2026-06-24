<script setup lang="ts">
import { Lock, User } from '@element-plus/icons-vue';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { showApiError } from '@/api/request';
import { useAuthStore } from '@/stores/auth';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const formRef = ref<FormInstance>();
const mode = ref<'login' | 'register'>('login');
const loading = ref(false);
const passwordFocused = ref(false);
const form = reactive({
  username: '',
  password: '',
  nickname: '',
});

// 鼠标跟随眼睛
const cloudRef = ref<SVGSVGElement | null>(null);
const eyeOffset = reactive({ lx: 0, ly: 0, rx: 0, ry: 0 });

function onMouseMove(e: MouseEvent) {
  if (!cloudRef.value) return;
  const rect = cloudRef.value.getBoundingClientRect();
  const cx = rect.left + rect.width / 2;
  const cy = rect.top + rect.height / 2;
  const dx = e.clientX - cx;
  const dy = e.clientY - cy;
  const dist = Math.sqrt(dx * dx + dy * dy);
  const maxShift = 4;
  const shift = Math.min(dist / 80, 1) * maxShift;
  const angle = Math.atan2(dy, dx);
  const ox = Math.cos(angle) * shift;
  const oy = Math.sin(angle) * shift;
  eyeOffset.lx = ox;
  eyeOffset.ly = oy;
  eyeOffset.rx = ox;
  eyeOffset.ry = oy;
}

onMounted(() => window.addEventListener('mousemove', onMouseMove));
onBeforeUnmount(() => window.removeEventListener('mousemove', onMouseMove));

const title = computed(() => (mode.value === 'login' ? '登录云盘' : '创建账号'));
const submitText = computed(() => (mode.value === 'login' ? '登录' : '注册'));

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 64, message: '用户名长度为 3-64 个字符', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 128, message: '密码长度为 6-128 个字符', trigger: 'blur' },
  ],
  nickname: [{ max: 128, message: '昵称不能超过 128 个字符', trigger: 'blur' }],
};

function toggleMode() {
  mode.value = mode.value === 'login' ? 'register' : 'login';
  formRef.value?.clearValidate();
}

async function submit() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) {
    return;
  }

  loading.value = true;
  try {
    if (mode.value === 'login') {
      await authStore.login({
        username: form.username,
        password: form.password,
      });
      ElMessage.success('登录成功');
      await router.replace(String(route.query.redirect || '/files'));
      return;
    }

    await authStore.register({
      username: form.username,
      password: form.password,
      nickname: form.nickname || undefined,
    });
    ElMessage.success('注册成功，请登录');
    mode.value = 'login';
    form.password = '';
  } catch (error) {
    showApiError(error, `${submitText.value}失败`);
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <main class="auth-page">
    <!-- 品牌卡片 -->
    <div class="auth-brand-card">
      <svg
        ref="cloudRef"
        class="cloud-avatar"
        viewBox="0 0 200 160"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <!-- 云朵轮廓：三段圆弧 + 底边水平直线 -->
        <path
          d="M30 120 C30 120, 10 120, 10 95 C10 70, 35 60, 50 65 C50 30, 85 10, 110 35 C125 15, 165 20, 170 55 C195 55, 195 85, 175 100 C190 110, 185 120, 170 120 Z"
          stroke="#000"
          stroke-width="3"
          fill="#fff"
        />

        <!-- 左眼 -->
        <template v-if="mode === 'login' && !passwordFocused">
          <circle cx="80" cy="80" r="12" fill="#fff" stroke="#000" stroke-width="2.5" />
          <circle :cx="80 + eyeOffset.lx" :cy="80 + eyeOffset.ly" r="5" fill="#000" />
        </template>
        <template v-else-if="mode === 'login' && passwordFocused">
          <!-- 闭眼：向下凸的圆弧 -->
          <path d="M68 82 Q80 94 92 82" stroke="#000" stroke-width="3" fill="none" stroke-linecap="round" />
        </template>
        <template v-else>
          <!-- 兴奋眼：左> 右< -->
          <polyline points="68,70 92,80 68,90" stroke="#000" stroke-width="3.5" fill="none" stroke-linecap="round" stroke-linejoin="round" />
        </template>

        <!-- 右眼 -->
        <template v-if="mode === 'login' && !passwordFocused">
          <circle cx="130" cy="80" r="12" fill="#fff" stroke="#000" stroke-width="2.5" />
          <circle :cx="130 + eyeOffset.rx" :cy="80 + eyeOffset.ry" r="5" fill="#000" />
        </template>
        <template v-else-if="mode === 'login' && passwordFocused">
          <!-- 闭眼：向下凸的圆弧 -->
          <path d="M118 82 Q130 94 142 82" stroke="#000" stroke-width="3" fill="none" stroke-linecap="round" />
        </template>
        <template v-else>
          <!-- 兴奋眼：左> 右< -->
          <polyline points="142,70 118,80 142,90" stroke="#000" stroke-width="3.5" fill="none" stroke-linecap="round" stroke-linejoin="round" />
        </template>
      </svg>

      <h1>个人云盘</h1>
      <p>管理你的文件、下载链接与云端资料</p>
    </div>

    <!-- 表单卡片 -->
    <section class="auth-form-card">
      <el-form
        ref="formRef"
        class="auth-form"
        :model="form"
        :rules="rules"
        label-position="top"
        @keyup.enter="submit"
      >
        <h2>{{ title }}</h2>

        <el-form-item label="用户名" prop="username">
          <el-input v-model.trim="form.username" placeholder="请输入用户名" size="large" :prefix-icon="User" />
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            placeholder="请输入密码"
            size="large"
            type="password"
            show-password
            :prefix-icon="Lock"
            @focus="passwordFocused = true"
            @blur="passwordFocused = false"
          />
        </el-form-item>

        <el-form-item v-if="mode === 'register'" label="昵称" prop="nickname">
          <el-input v-model.trim="form.nickname" placeholder="可选，默认使用用户名" size="large" />
        </el-form-item>

        <el-button type="primary" size="large" :loading="loading" class="full-button" @click="submit">
          {{ submitText }}
        </el-button>

        <el-button text class="switch-button" @click="toggleMode">
          {{ mode === 'login' ? '还没有账号？去注册' : '已有账号？去登录' }}
        </el-button>
      </el-form>
    </section>
  </main>
</template>
