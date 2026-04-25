<script setup lang="ts">
import { Lock, User } from '@element-plus/icons-vue';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { computed, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { showApiError } from '@/api/request';
import { useAuthStore } from '@/stores/auth';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const formRef = ref<FormInstance>();
const mode = ref<'login' | 'register'>('login');
const loading = ref(false);
const form = reactive({
  username: '',
  password: '',
  nickname: '',
});

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
    <section class="auth-panel">
      <div class="auth-brand">
        <div class="brand-mark">云</div>
        <div>
          <h1>个人云盘</h1>
          <p>管理你的文件、下载链接与云端资料</p>
        </div>
      </div>

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
