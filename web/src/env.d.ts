/// <reference types="vite/client" />

/**
 * Vue 单文件模块类型声明
 * 允许 TS 正确识别 .vue 文件的默认导出组件类型
 */
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
