/// <reference types="vite/client" />

import type { DefineComponent } from 'vue';

// 声明项目使用到的 Vite 环境变量，便于 TypeScript 做类型检查和补全。
interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string;
}

// 扩展 import.meta.env 的类型定义。
interface ImportMeta {
  readonly env: ImportMetaEnv;
}

// 让 TypeScript 语言服务识别 .vue 单文件组件导入。
declare module '*.vue' {
  const component: DefineComponent<object, object, unknown>;
  export default component;
}
