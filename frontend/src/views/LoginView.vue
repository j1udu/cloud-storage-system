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

// 登录和注册共用一套表单状态，通过 mode 控制当前提交行为。
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

// Element Plus 表单校验规则，先在前端拦截明显不合法的输入。
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
  // 切换登录/注册时清掉上一种模式留下的校验提示。
  mode.value = mode.value === 'login' ? 'register' : 'login';
  formRef.value?.clearValidate();
}

async function submit() {
  // validate 返回失败时会抛错，这里统一转换成 false，避免进入提交流程。
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
      // 如果是从受保护页面跳来的，登录后回到原页面；否则进入文件页。
      await router.replace(String(route.query.redirect || '/files'));
      return;
    }

    await authStore.register({
      username: form.username,
      password: form.password,
      nickname: form.nickname || undefined,
    });
    ElMessage.success('注册成功，请登录');
    // 当前产品注册后不自动登录，让用户回到登录模式重新提交。
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
      <!-- 左侧品牌区负责建立产品语境，右侧表单负责实际认证流程。 -->
      <div class="auth-brand">
        <div class="brand-mark">云</div>
        <div>
          <h1>个人云盘</h1>
          <p>管理你的文件、下载链接与云端资料</p>
        </div>
      </div>

      <!-- Element Plus 表单通过 ref 暴露 validate/clearValidate 等方法。 -->
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

        <!-- 只有注册模式需要昵称；登录接口不需要这个字段。 -->
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
