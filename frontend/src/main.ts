import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';

import { createPinia } from 'pinia';
import { createApp } from 'vue';

import App from './App.vue';
import router from './router';
import './styles.css';

// 前端应用的统一装配入口：在这里集中注册全局插件和根组件。
createApp(App).use(createPinia()).use(router).use(ElementPlus).mount('#app');
