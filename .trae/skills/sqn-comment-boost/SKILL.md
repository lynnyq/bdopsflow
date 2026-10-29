---
name: sqn-comment-boost
description: SonarQube 前端注释率提升与体检技能。用于量化评估 Vue 项目注释覆盖率 批量补充注释 以及多维度体检扫描防止批量脚本引入代码污染。当用户提到注释率不足 SonarQube 报警 comment density 时触发。不用于写业务功能。
---

# SonarQube 注释率提升技能

将 Vue 项目前端注释率从不足 10% 提升到 SonarQube 要求的 15%+，并在全程进行污染体检。

## 何时使用

- SonarQube 报告前端注释率不足 15%
- 批量补充 Vue/TS 文件注释后做质量体检
- 用户要求提升前端代码注释覆盖率

## 前置条件

- 前端项目已可正常 `npm run build`
- 项目使用 Vue 3 + TypeScript（对 Vue 2 仅部分适用）
- 有 Python 3 环境（用于注释率统计和体检脚本）

## 工作流程

### 阶段 1：量化评估

运行注释率统计脚本，得到：

- 项目整体注释率（SonarQube 统计口径：独立注释行 / 总行数）
- 注释率最低的 TOP 20 文件及缺口行数
- 注释率低于 15% 的文件数和总缺口行数

**评估命令**：
```bash
cd <前端源码目录> && python3 <skill>/scripts/comment-rate.py
```

### 阶段 2：批量补充（三层策略）

按优先级（总行数 × 缺口百分比）分批处理文件。对每个文件执行：

#### 第一层：Script JSDoc（主力，性价比最高）

- 每个独立函数（`const xxx = async (...) =>` / `function xxx()`）加完整 JSDoc：
  ```js
  /**
   * 功能描述（中文，一句话）
   * @param {string} paramName - 参数含义（中文）
   * @returns {Promise<void>} 返回值含义
   */
  async function loadTasks(page = 1) { ... }
  ```
- 每个 `ref` 变量加行内注释：`const loading = ref(false)  // 加载状态`
- 每个 `reactive` 对象字段加行内注释：`username: '',  // 用户名`
- `defineProps` / `defineEmits` 加 JSDoc
- 用节注释组织代码：`// ========== 数据加载 ==========`、`// ========== 事件处理 ==========`
- **跳过** 已存在 JSDoc 的函数（不要重复添加）

#### 第二层：Template 区块注释

- `<template>` 顶部加组件功能描述：
  ```html
  <!--
    XXX 页面/组件
    功能：一句话描述
    主要区域：...
  -->
  ```
- 每个大型 UI 区块前加分节注释：`<!-- === 区块名称 === -->`
- 不要在 HTML 标签的属性列表中间插入注释（Vue 解析器不允许）

#### 第三层：CSS 区块注释

- `<style>` 主要选择器前加：`/* === .className 功能说明 === */`

### 阶段 3：中间验证

每完成 3-5 个大文件后执行一次：

```bash
cd <前端项目根> && npm run build 2>&1 | tail -5
cd <前端项目根> && npx vue-tsc --noEmit 2>&1 | grep -E "error TS"
```

### 阶段 4：终极体检（关键，防止批量脚本污染）

**这是从多次踩坑中总结的必做步骤**。运行体检脚本：

```bash
python3 <skill>/scripts/health-check.py <前端源码目录>
```

体检脚本检查以下 6 类常见污染：

| 检查项 | 说明 | 真实案例 |
|--------|------|---------|
| 🔴 Python 关键字残留 | `None`/`True`/`False`/`pass` 被误注入 JS | Python 脚本批量处理后留下 39 处 |
| 🔴 JSDoc 插在 `import {}` 中间 | 文件级 JSDoc 被 Python 脚本插入 import 块 | Tasks.vue 的图标 import 被切断 |
| 🔴 HTML 注释插在标签属性中间 | `<!-- 注释 -->` 夹在 `<div ref="...">` 属性之间 | HttpTest.vue 导致 Vue 模板解析报错 |
| 🔴 `:class` / `:style` 对象尾逗号 | Vue 模板中对象字面量不应有尾逗号 | HttpTest.vue 导致运行时错误 |
| 🔴 Script 区块泄漏 HTML 标签 | `</template>` 等误出现在 JS 中 | — |
| 🔴 Style 区块泄漏 JS 代码 | `import {}` / `function` 误出现在 CSS 中 | — |

### 阶段 5：收尾验证

```bash
# 1. 注释率确认（必须 ≥ 15%）
cd <前端源码目录> && python3 <skill>/scripts/comment-rate.py

# 2. 构建确认
cd <前端项目根> && npm run build

# 3. 类型检查确认
cd <前端项目根> && npx vue-tsc --noEmit
```

## 铁律

1. **永远只 Edit 不 Write** — 批量脚本可能破坏文件，用 Edit 工具逐段添加注释
2. **先 Read 再 Edit** — 确认 old_string 精确匹配当前文件内容（包括已有的注释）
3. **不修改任何现有代码** — 不删除、重排、替换任何非注释行
4. **注释全部用中文** — 与项目 AGENTS.md 风格一致
5. **不覆盖现有注释** — 已存在 JSDoc/行内注释的跳过
6. **Python 脚本只用于统计和体检** — 不要用 Python 脚本直接改写 Vue/TS 文件内容（已多次导致污染）
7. **每轮批量后必须跑体检** — 哪怕只处理了 3 个文件
