# WebUI - Admin Dashboard for Wistia-S3

## Goal / Background

Build an admin WebUI at route `/ui` for managing Wistia-to-S3 video migrations. The current interface is Swagger only; this UI provides a business-minimal dashboard for daily operations: browsing media, triggering migrations, monitoring tasks, and viewing AI indexing results.

## Technical Approach

### Architecture: 複用 web/ rspack 架構，獨立輸出目錄

WebUI 作為 `web/` 子項目的第二個 namespace，與現有 `player` 並列。共用 `web/` 目錄和 `yarn` 指令，通過 **prompt (dev) / env var (build)** 選擇 namespace，各自獨立輸出:

- `NAMESPACE=player` → `web/dist/` (現有，不變)
- `NAMESPACE=webui` → `webroot/webui/` (新增)
- 不指定 → build both

```
web/                          (現有)
  rspack.config.js            (修改: factory functions + prompt/env namespace 選擇)
  postcss.config.mjs          (新增 - TailwindCSS v4 PostCSS 配置, 僅 uiConfig 使用)
  package.json                (修改: 新增 vue, tailwindcss, vue-loader 等依賴)
  src/
    main.js                   (現有 - wistia-s3 player entry)
    demo.js                   (現有 - demo entry)
    index.html                (現有 - player HTML template)
    demo.html                 (現有 - demo HTML template)
    ui/                       (新增 - WebUI namespace)
      main.js                 (Vue 3 app entry)
      index.html              (SPA shell, 無 Go template vars)
      style.css               (TailwindCSS v4 entry: @import "tailwindcss")
      App.vue
      api/
        index.js              (fetch wrappers for all API endpoints)
      composables/
        useTaskPolling.js     (poll GET /tasks/{id} until finished/error)
      components/
        AppLayout.vue         (mobile: 漢堡選單, desktop: 側邊欄)
        MediaCard.vue         (video card with thumbnail, name, duration)
        TaskBadge.vue         (status badge: init/running/finished/error)
        ConfirmDialog.vue     (reusable confirm modal)
        Toast.vue             (toast notification)
      views/
        MediaLibrary.vue      (main page: video grid + search + actions)
        VideoDetail.vue       (single video: metadata, assets, AI index)
        Tasks.vue             (task monitoring page)
      router/
        index.js              (Vue Router - hash mode)
  dist/                       (現有 - player/demo output, 不變)
    wistia-s3.min.js
    demo.min.js
    index.html
    demo.html

webroot/
  swagger/                    (現有)
  webui/                      (新增 - WebUI output, Go serves here)
    index.html                (SPA shell)
    ui.min.js                 (Vue bundle, inlined into index.html)
```

#### URL mapping

```
Browser:  /ui          → Go redirect → /ui/index.html
Browser:  /ui/index.html → Go static → webroot/webui/index.html
Vue hash: /ui/index.html#/media
          /ui/index.html#/video/abc123
          /ui/index.html#/tasks
```

### Stack

| 項目 | 選擇 | 說明 |
|------|------|------|
| Framework | Vue 3 (Composition API + `<script setup>`) | SFC 模式 |
| Build | rspack (現有) | prompt/env 選擇 namespace |
| CSS | TailwindCSS v4 (build-time) | PostCSS plugin 方式整合 |
| Router | Vue Router 4 | Hash mode (`#/media`, `#/video/:hash`, `#/tasks`) |
| HTTP | Vanilla fetch | 不加 Axios，保持依賴最小 |
| State | reactive + composables | 不加 Pinia，依赖少 |
| UI 語言 | 繁體中文 | 所有 UI 文字、按鈕、placeholder |
| 響應式 | Mobile-first | TailwindCSS 預設斷點，mobile → desktop |

### rspack.config.js 修改要點

沿用現有 prompt 模式，根據 namespace 選擇返回不同 config。定義兩個 config factory function，通過 prompt (dev) 或 env var (build) 選擇。

#### 1. Config factory functions

```js
// rspack.config.js 頂部 imports
const { VueLoaderPlugin } = require('rspack-vue-loader');  // rspack fork, 無 webpack 依賴

// 現有 baseConfig 重構為 function
function getPlayerConfig() {
  return {
    mode: IS_DEV ? 'development' : 'production',
    experiments: { css: true },
    entry: {
      'wistia-s3': ['./src/main.js'],
      'demo': ['./src/demo.js'],
    },
    output: {
      filename: IS_DEV ? '[name].js' : '[name].min.js',
      path: path.resolve(__dirname, 'dist'),
      publicPath: 'auto',
      clean: true,
    },
    plugins: [
      new rspack.ProgressPlugin(),
      new rspack.HtmlRspackPlugin({ /* demo.html */ }),
      new rspack.HtmlRspackPlugin({ /* index.html */ }),
      new rspack.CopyRspackPlugin({ /* favicon */ }),
      // dev/production plugins
      IS_DEV ? new DevTemplatePlugin() : new InlineJSPlugin(),
    ],
    // ... 其餘不變
  }
}

function getUiConfig() {
  return {
    mode: IS_DEV ? 'development' : 'production',
    experiments: { css: true },
    entry: {
      ui: ['./src/ui/main.js'],
    },
    output: {
      filename: IS_DEV ? '[name].js' : '[name].min.js',
      path: path.resolve(__dirname, '../webroot/webui'),
      publicPath: '/ui/',
      clean: true,
    },
    plugins: [
      new rspack.ProgressPlugin(),
      new rspack.HtmlRspackPlugin({
        filename: 'index.html',
        template: path.resolve(__dirname, 'src/ui/index.html'),
        inject: 'body',
        minify: !IS_DEV,
      }),
      new VueLoaderPlugin(),  // from 'rspack-vue-loader'
    ],
    module: {
      rules: [
        {
          test: /\.vue$/,
          loader: 'rspack-vue-loader',
          options: { experimentalInlineMatchResource: true },
        },
        {
          test: /\.css$/,
          use: ['postcss-loader'],  // postcss.config.mjs handles @tailwindcss/postcss
          type: 'css',
        },
        {
          test: /\.(png|jpe?g|gif|svg|webp|ico|eot|ttf|otf|woff2?)$/i,
          type: 'asset',
          generator: { filename: 'assets/[hash][ext][query]' },
          parser: { dataUrlCondition: { maxSize: 200 * 1024 } },
        },
      ],
    },
    optimization: { splitChunks: false, runtimeChunk: false, minimize: !IS_DEV },
    performance: { hints: false },
    devtool: 'source-map',
    devServer: {
      open: true,
      compress: true,
      hot: true,
      static: { directory: path.join(__dirname, '../webroot/webui') },
      proxy: [
        { context: ['/media', '/move', '/refresh', '/tasks', '/index'], target: 'http://localhost:8080' },
      ],
    },
  }
}
```

#### 2. Prompt-based namespace selection (dev mode)

```js
// Dev mode: prompt 選擇 namespace
if (IS_DEV) {
  module.exports = prompt([
    {
      type: 'list',
      name: 'namespace',
      message: '選擇開發項目',
      choices: [
        { name: 'Player (wistia-s3 / demo)', value: 'player' },
        { name: 'WebUI (admin dashboard)', value: 'webui' },
      ],
    },
    {
      type: 'list',
      name: 'public',
      message: '是否允許外網訪問',
      choices: [{ name: '允許', value: true }, { name: '不需要', value: false }],
      when: ({ namespace }) => namespace === 'player',  // 只有 player 需要 FRP
    },
    {
      type: 'input',
      name: 'subdomain',
      message: '請配一個域名',
      validate: (input) => /^([a-z0-9\-]{4,})$/i.test(input),
      when: ({ namespace, public: p }) => namespace === 'player' && p,
    },
  ]).then(({ namespace, public: usePublic, subdomain }) => {
    if (namespace === 'webui') {
      return getUiConfig()
    }
    // player: 套用 FRP devServer 設定
    const config = getPlayerConfig()
    if (usePublic && subdomain) {
      config.devServer = { /* FRP 設定，同現有邏輯 */ }
    }
    return config
  })
}
```

#### 3. Production build (env var 選擇)

```js
// Production: 根據 NAMESPACE env var 選擇 build 目標
if (!IS_DEV) {
  const ns = process.env.NAMESPACE
  if (ns === 'webui') {
    module.exports = getUiConfig()
  } else if (ns === 'player') {
    module.exports = getPlayerConfig()
  } else {
    // 不指定 NAMESPACE → build both (multi-compiler)
    module.exports = [getPlayerConfig(), getUiConfig()]
  }
}
```

#### 4. package.json scripts

```jsonc
{
  "scripts": {
    "build": "cross-env NODE_ENV=production rspack build",           // build both
    "build:player": "cross-env NODE_ENV=production NAMESPACE=player rspack build",
    "build:webui": "cross-env NODE_ENV=production NAMESPACE=webui rspack build",
    "serve": "cross-env NODE_ENV=development rspack serve"          // prompt 選擇
  }
}
```

#### 5. postcss.config.mjs (新增)

```js
// web/postcss.config.mjs
// 此配置僅影響 uiConfig 的 CSS (playerConfig 不使用 postcss-loader)
export default {
  plugins: {
    '@tailwindcss/postcss': {},
  },
}
```

#### 6. 注意事項

- `InlineJSPlugin` / `DevTemplatePlugin` 只在 `getPlayerConfig()` 中使用，`getUiConfig()` 不需要
- uiConfig 的 `devServer.proxy` 直接 proxy API routes 到 Go backend，不需要 FRP
- `LISTEN` env 決定 Go port（預設未設定，需確認）。proxy target 應可配置。
- Dev 模式下两个 namespace 各自獨立 dev server，不能同時運行（port 衝突）。如需同時開發，可改 port。

### Go backend 整合

#### pkg/http.go 路由修改

```go
// 新增 /ui 路由 - 必須在 NotFoundHandler 之前
r.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
    http.Redirect(w, r, "/ui/index.html", http.StatusFound)
})
r.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/",
    http.FileServer(http.Dir(fmt.Sprintf("%s/webui", s.config.Webroot)))))
```

> **SPA routing**: 使用 Vue Router hash mode (`#/media`, `#/video/xxx`)，所有路由在 `index.html` 內完成，Go 只需 serve 靜態文件，不需要 history fallback。

#### webroot 目錄結構

```
webroot/
  swagger/          (現有)
  webui/            (新增 - rspack uiConfig 直接輸出)
    index.html      (SPA shell)
    ui.min.js       (Vue bundle, inlined)
```

> rspack `uiConfig` 直接輸出到 `webroot/webui/`，無需額外 copy 步驟。

### TailwindCSS v4 設計規範 (Business-minimal, Mobile-first)

#### 響應式策略 (Mobile-first)

- **斷點**: 使用 TailwindCSS 預設斷點，mobile-first 寫法
  - 預設: mobile (< 640px)
  - `sm:`: 640px+
  - `md:`: 768px+
  - `lg:`: 1024px+
  - `xl:`: 1280px+
- **佈局原則**:
  - Mobile: 單欄堆疊，全寬元件
  - Tablet (md+): 雙欄佈局，側邊欄可收合
  - Desktop (lg+): 固定側邊欄 + 主內容區
- **表格響應式**:
  - Mobile: 卡片列表（每個 item 獨立卡片）
  - Desktop: 傳統表格
- **導航**:
  - Mobile: 漢堡選單 / 底部導航
  - Desktop: 側邊欄

#### 色彩

| 用途 | Class | 說明 |
|------|-------|------|
| Background | `bg-white` | 主背景 |
| Surface | `bg-slate-50` | 卡片/面板背景 |
| Border | `border-slate-200` | 分隔線 |
| Text primary | `text-slate-900` | 標題 |
| Text secondary | `text-slate-500` | 說明文字 |
| Accent | `text-blue-600` / `bg-blue-600` | 主要按鈕/連結 |
| Success | `text-emerald-600` / `bg-emerald-50` | finished 狀態 |
| Error | `text-red-600` / `bg-red-50` | error 狀態 |
| Warning | `text-amber-600` / `bg-amber-50` | running 狀態 |
| Info | `text-slate-500` / `bg-slate-100` | init 狀態 |

#### 排版原則

- System font stack (TailwindCSS 預設)
- 無 gradient、無裝飾性 shadow（僅 `shadow-sm` 用於卡片浮起）
- 緊湊間距: `p-3`, `gap-3`, `space-y-2`
- 邊框取代 shadow: `border border-slate-200 rounded`
- 表格用 `text-sm`，卡片用 `text-base`
- 按鈕: `px-3 py-1.5 text-sm font-medium rounded border`
- 過渡: `transition-colors duration-150`（無其他動畫）
- **觸控友善**: 按鈕最小 44x44px (`min-h-11 min-w-11`)

### 路由設計 (Vue Router - Hash Mode)

| Hash Route | View | 說明 |
|------------|------|------|
| `#/media` | MediaLibrary | 預設頁，視頻列表 |
| `#/video/:hash` | VideoDetail | 單一視頻詳情 |
| `#/tasks` | Tasks | 任務監控 |

> Hash mode 原因: Go 只需 serve `index.html` 靜態文件，無需處理 SPA history fallback。

## API Endpoints Used

| Endpoint | Method | UI Usage |
|----------|--------|----------|
| `/media` | GET | Media library list (all videos) |
| `/media?hash={hash}` | GET | Video detail (single video) |
| `/move/{hash}` | POST | Single video migration |
| `/move` | POST | Batch migration `{"media": ["hash1", ...]}` |
| `/move/{hash}?forceRefresh=true` | POST | Force re-migrate |
| `/refresh/media` | POST | Re-index S3 to local DB |
| `/tasks/{id}` | GET | Poll task status |
| `/index/{hash}` | POST | Trigger AI indexing |
| `/index/{hash}?force=true` | POST | Force re-index |
| `/index/{hash}` | GET | Get AI index result |
| `/index` | POST | Batch AI indexing `{"media": ["hash1", ...]}` |

## Pages / Views 詳細設計 (繁體中文 UI)

### 1. MediaLibrary (`#/media`)

#### Mobile (< 768px)
```
┌─────────────────────────┐
│ ☰ 視頻管理               │
├─────────────────────────┤
│ [🔍 搜尋 hash 或名稱...] │
├─────────────────────────┤
│ [整理] [刷新S3] [遷移]   │
├─────────────────────────┤
│ ┌─────────────────────┐ │
│ │ ☐ 視頻名稱           │ │
│ │ hash: abc123 2:30   │ │
│ │ [遷移] [AI索引]      │ │
│ └─────────────────────┘ │
│ ┌─────────────────────┐ │
│ │ ☐ 視頻名稱 2         │ │
│ │ hash: def456 1:45   │ │
│ │ [遷移] [AI索引]      │ │
│ └─────────────────────┘ │
│ ...                     │
├─────────────────────────┤
│ < 1 2 3 ... >           │
└─────────────────────────┘
```

#### Desktop (≥ 768px)
```
┌─────────────────────────────────────────────────┐
│ ☰ 視頻管理                                       │
├─────────────────────────────────────────────────┤
│ [🔍 搜尋 hash 或名稱...]  [刷新S3] [批量遷移]     │
├─────────────────────────────────────────────────┤
│ ☑ 全選                                (12 項目)  │
├────────┬────────┬────────┬────────┬─────────────┤
│ ☐      │ 縮圖   │ 名稱   │ Hash   │ 操作        │
│        │        │        │        │ [遷移]      │
│        │        │ 2:30   │ abc123 │ [AI索引]    │
├────────┼────────┼────────┼────────┼─────────────┤
│ ☐      │ 縮圖   │ 名稱2  │ def456 │ ...         │
└────────┴────────┴────────┴────────┴─────────────┘
│ < 1 2 3 ... >                                    │
└─────────────────────────────────────────────────┘
```

- **Mobile**: 卡片列表，每個視頻獨立卡片，操作按鈕在卡片內
- **Desktop**: 傳統表格，checkbox 多選
- 搜尋: 前端 filter by name or hash (debounce 300ms)
- Toolbar actions:
  - "刷新S3" → POST `/refresh/media` → toast + poll task
  - "批量遷移" → POST `/move` with selected hashes → toast + poll task
  - "批量AI索引" → POST `/index` with selected hashes
- Row/Card click → navigate to `#/video/:hash`
- Pagination: 前端分頁，每頁 20 筆

### 2. VideoDetail (`#/video/:hash`)

#### Mobile
```
┌─────────────────────────┐
│ ← 返回視頻列表           │
├─────────────────────────┤
│ 視頻名稱                 │
│ abc123xyz | 2:30        │
├─────────────────────────┤
│ [遷移到S3] [強制重新遷移]│
│ [AI索引]    [強制重建索引]│
├─────────────────────────┤
│ ▼ 素材列表               │
│ ┌─────────────────────┐ │
│ │ mp4 12MB 1920x1080  │ │
│ │ mp4 8MB  1280x720   │ │
│ │ jpg 120K 640x360    │ │
│ └─────────────────────┘ │
├─────────────────────────┤
│ ▼ AI 索引結果            │
│ 摘要: ...               │
│ ▼ 章節 (3)              │
│  0:00 簡介              │
│  1:23 開始              │
│ ▼ 字幕 (20)             │
│  0:00-0:05 你好         │
│ Token: 12.3K in        │
└─────────────────────────┘
```

#### Desktop
```
┌─────────────────────────────────────────────────┐
│ ← 返回視頻列表                                   │
├─────────────────────────────────────────────────┤
│ 視頻名稱                              [狀態]     │
│ Hash: abc123xyz  |  時長: 2:30  |  ID: 123      │
├─────────────────────────────────────────────────┤
│ 操作:                                            │
│ [遷移到S3] [強制重新遷移]                         │
│ [AI索引]   [強制重建索引]                         │
├──────────────────────┬──────────────────────────┤
│ 素材列表              │ AI 索引結果               │
│                      │                           │
│ 類型  | 大小 | 尺寸   │ 摘要: ...                 │
│ mp4   | 12MB |1920x │                           │
│ mp4   | 8MB  |1280x │ 章節:                      │
│ jpg   | 120K | 640x │  0:00 - 簡介              │
│                      │  1:23 - 開始              │
│                      │                           │
│                      │ 字幕 (20 筆)               │
│                      │ 0:00-0:05  你好世界        │
│                      │ 0:05-0:10  歡迎來到...     │
│                      │                           │
│                      │ Token 用量: 12.3K 輸入     │
│                      │   4.5K 輸出  16.8K 總計    │
└──────────────────────┴──────────────────────────┘
```

- **Mobile**: 單欄堆疊，素材和 AI 結果用可摺疊區塊
- **Desktop**: 雙欄佈局（左: 素材，右: AI 結果）
- 若無 AI index: 顯示 "尚無 AI 索引" + "生成索引" 按鈕
- Actions 觸發後: toast notification + 自動 poll task status

### 3. Tasks (`#/tasks`)

#### Mobile
```
┌─────────────────────────┐
│ 任務監控        (自動 3s) │
├─────────────────────────┤
│ ┌─────────────────────┐ │
│ │ 1234.. ● 執行中      │ │
│ │ 10:23:01             │ │
│ │ 處理中...            │ │
│ └─────────────────────┘ │
│ ┌─────────────────────┐ │
│ │ 5678.. ✓ 完成        │ │
│ │ 10:22:45             │ │
│ │ 3/3 成功             │ │
│ └─────────────────────┘ │
└─────────────────────────┘
```

#### Desktop
```
┌─────────────────────────────────────────────────┐
│ 任務監控                              (自動 3s)  │
├────────┬──────────┬──────────┬──────────────────┤
│ 任務ID │ 狀態     │ 建立時間  │ 結果             │
├────────┼──────────┼──────────┼──────────────────┤
│ 1234.. │ ● 執行中 │ 10:23:01 │ 處理中...        │
│ 5678.. │ ✓ 完成   │ 10:22:45 │ 3/3 成功         │
│ 9012.. │ ✗ 錯誤   │ 10:21:30 │ 逾時             │
└────────┴──────────┴──────────┴──────────────────┘
```

- **Mobile**: 任務卡片列表
- **Desktop**: 傳統表格
- **注意**: Tasks 存於 Go 記憶體 (`map[string]*Task`)，頁面刷新後丟失
- UI 維護一個本地 task list (Vue reactive array)，新 task 加入後自動 poll
- 狀態 badge: init (灰), running (黃 + spinner), finished (綠), error (紅)
- 每 3 秒 poll running tasks，finished/error 後停止 poll
- Result 展開: 可點擊查看 `MoveToS3Result[]` 詳情

## 新增依賴 (package.json)

```jsonc
{
  "dependencies": {
    // 現有
    "clipboard": "^2.0.11",
    "highlight.js": "^11.10.0",
    "wistia-s3-player": "^1.2.3"
    // 不新增 runtime deps (Vue 在 devDependencies)
  },
  "devDependencies": {
    // 現有
    "@rspack/cli": "^1.4.10",
    "@rspack/core": "^1.4.10",
    "cross-env": "^7.0.3",
    "dotenv": "^16.4.5",
    "inquirer": "^8.2.6",
    "mmhk-frp": "^1.1.0",
    // 新增
    "vue": "^3.5",
    "vue-router": "^4.5",
    "rspack-vue-loader": "^17.4",     // rspack fork, 無 webpack 依賴
    "tailwindcss": "^4.0",
    "@tailwindcss/postcss": "^4.0",
    "postcss": "^8.4",
    "postcss-loader": "^8.1"
  }
}
```

## Task Breakdown

### Phase 1: 基礎架構
- [ ] 1. 安裝新依賴 (vue, vue-router, rspack-vue-loader, tailwindcss v4, @tailwindcss/postcss, postcss, postcss-loader)
- [ ] 2. 建立 `postcss.config.mjs` (TailwindCSS v4 PostCSS 配置)
- [ ] 3. 重構 `rspack.config.js`: 抽取 `getPlayerConfig()` / `getUiConfig()` factory functions
- [ ] 4. 修改 prompt 流程: 新增 namespace 選擇，FRP prompts 限定 player namespace
- [ ] 5. 修改 production export: 根據 `NAMESPACE` env var 選擇 config
- [ ] 6. 建立 `src/ui/` 目錄結構 + Vue app entry + router (hash mode)
- [ ] 7. 建立 AppLayout.vue (mobile: 漢堡選單, desktop: 側邊欄 + header + router-view)

### Phase 2: API + Composables
- [ ] 7. 實作 `src/ui/api/index.js` (fetch wrappers for all endpoints)
- [ ] 8. 實作 `useTaskPolling` composable
- [ ] 9. 實作 Toast composable / component

### Phase 3: Views
- [ ] 10. MediaLibrary view (table + search + selection + pagination)
- [ ] 11. VideoDetail view (metadata + assets + AI index results)
- [ ] 12. Tasks view (table + auto-refresh)
- [ ] 13. 共用 components (TaskBadge, ConfirmDialog)

### Phase 4: Go Backend + Build
- [ ] 14. 修改 `pkg/http.go`: 新增 `/ui` redirect + `/ui/` static file route (→ `webroot/webui/`)
- [ ] 15. 更新 `.gitignore`: 加入 `webroot/webui/`
- [ ] 16. 更新 `package.json` scripts: 新增 `build:player`, `build:webui`
- [ ] 17. Build verification: `yarn build` → `docker build` → 手動測試

## Affected Files

### New files
- `web/src/ui/` (entire directory - Vue SPA source)
- `web/postcss.config.mjs` (TailwindCSS v4 PostCSS 配置)
- `webroot/webui/` (build output, gitignored)

### Modified files
- `web/rspack.config.js` — 重構為 factory functions, prompt namespace 選擇, env var build 選擇
- `web/postcss.config.mjs` — 新增 TailwindCSS v4 PostCSS 配置
- `web/package.json` — 新增 vue, rspack-vue-loader, tailwindcss 等依賴 + `build:player`, `build:webui` scripts
- `pkg/http.go` — 新增 `/ui` + `/ui/` routes (→ `webroot/webui/`)
- `.gitignore` — 新增 `webroot/webui/`

### Unchanged files
- `web/src/main.js`, `web/src/demo.js` — 現有 player entries 不受影響
- `web/src/index.html`, `web/src/demo.html` — 現有 HTML templates 不受影響
- `web/dist/` — player/demo output 目錄不變
- `InlineJSPlugin`, `DevTemplatePlugin` — 只在 `getPlayerConfig()` 中使用，不受影響

## Gotchas / 風險

1. **Prompt 流程改動**: 現有 dev prompt 只問 FRP 設定，需新增 namespace 選擇。FRP prompts 應只在 `namespace === 'player'` 時顯示 (`when` condition)。

2. **rspack-vue-loader**: 使用 rspack fork 的 `rspack-vue-loader` (非 webpack 的 `vue-loader`)。需設定 `experimentalInlineMatchResource: true` 以支援完整功能。`VueLoaderPlugin` 從 `rspack-vue-loader` import。

3. **TailwindCSS v4 + PostCSS**: uiConfig 使用 `postcss-loader` + `@tailwindcss/postcss` (via `postcss.config.mjs`)。playerConfig 不使用 PostCSS，維持 rspack 原生 CSS 處理。兩個 config 的 CSS 處理互不干擾。

4. **Vue SFC `<style>` 區塊**: Vue SFC 的 `<style>` 會透過 `rspack-vue-loader` + `postcss-loader` 處理，與 `style.css` 使用相同的 TailwindCSS 配置。

5. **SPA routing**: 使用 hash mode 避免 Go 端 SPA fallback 問題。URL 格式: `/ui/index.html#/media`。

6. **Tasks 是記憶體存儲**: Go 重啟後所有 task 丟失。UI 的 task list 也是 Vue reactive state，刷新頁面後丟失。這是已知限制。

7. **publicPath 設定**: uiConfig 的 `output.publicPath` 設為 `/ui/`，確保 Go serve 時能正確載入資源。playerConfig 維持 `publicPath: 'auto'`。

8. **Dev server 不能同時運行**: player 和 webui 的 dev server 使用不同 port，不能同時透過 `yarn serve` 運行。如需同時開發，需開兩個 terminal 並指定不同 port。

## Testing Strategy

- **後端**: 所有 integration tests 不受影響（`go test ./pkg/...`）
- **前端**: 
  - `yarn build` 必須成功，輸出 `webroot/webui/index.html` + `webroot/webui/ui.min.js`
  - `yarn build:player` 必須成功，輸出 `web/dist/` (現有行為不變)
  - `yarn build:webui` 必須成功，輸出 `webroot/webui/`
- **手動驗證**:
  1. `yarn serve` → 選擇 WebUI → 確認 SPA 載入
  2. 確認 `#/media`, `#/video/:hash`, `#/tasks` 頁面正常渲染
  3. 確認 API calls 透過 dev proxy 正確轉發
  4. `docker build` → 訪問 `/ui/` → 確認 production build 正常
  5. **響應式測試**: Chrome DevTools 模擬 mobile (375px), tablet (768px), desktop (1280px)
- **現有功能**: 確認 `yarn serve` 選擇 Player 後，`index.html` 和 `demo.html` 不受影響

## Open Questions

無
