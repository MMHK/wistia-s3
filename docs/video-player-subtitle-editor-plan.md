# Video Player + Subtitle Alignment Editor 實施方案

**Created**: 2026-06-19
**Status**: 🟢 Complete
**Priority**: High

---

## 1. 背景與目標

### 現狀
- `VideoDetail.vue` 目前只顯示 metadata 和 AI 索引文字列表
- 沒有播放器可以預覽影片
- 字幕只能以文字列表顯示，無法編輯或對齊

### 目標
1. **嵌入 Wistia Player**：在 VideoDetail 頁面最上方嵌入播放器，可播放 S3 上的影片
2. **字幕對齊編輯 UI**：在 Player 下方新增表格編輯器，可編輯 start/end/text 並儲存回 S3 + BoltDB
3. **Player 與字幕互動**：點擊字幕跳轉播放、播放時高亮當前字幕

---

## 2. Player 嵌入方式分析

### 2.1 Demo Page 嵌入模式

從 demo.html 和 index.html 模板分析，player 嵌入方式如下：

```html
<!-- Responsive wrapper -->
<div class="wistia_responsive_padding">
    <div class="wistia_responsive_wrapper">
        <div class="wistia_embed wistia_async_{hashId} videoFoam=true playsinline=true" 
             style="height:100%;width:100%">&nbsp;</div>
    </div>
</div>

<!-- Player script -->
<script type="text/javascript" src="{WistiaS3JSUrl}"></script>
```

### 2.2 Player 初始化流程

`wistia-s3-player` 套件（`wistia-s3.min.js`）的初始化邏輯：

```js
// 1. 掃描 DOM 中的 .wistia_embed 元素
document.querySelectorAll(".wistia_embed").forEach((el) => {
    // 2. 從 class 提取 hashId: wistia_async_{hashId}
    const matches = el.className.match(/wistia_async_([^_\ ]+)/i);
    if (matches) {
        const id = matches[1];
        // 3. 建立 Vue 3 app 並 mount
        const app = createApp(WistiaPlayer, { id, MeasurementId: GA_ID });
        app.mount(el);
    }
})
```

### 2.3 Player 功能

`wistia-s3-player` 基於 video.js，提供：
- 多解析度切換（從 S3 的 `index.json` 讀取不同高度的 MP4）
- 字幕支援（載入 `subtitles.vtt`）
- 章節標記（從 `index-ai.json` 讀取 chapters）
- 自訂控制列（播放/暫停、播放速度、畫質選擇、CC 開關）
- 縮圖預覽（hover 進度條時顯示 sprite 圖）
- GA 追蹤

### 2.4 Admin UI 嵌入方案

**方案：動態載入 Player Script**

由於 admin UI 是 Vue SPA，需要：

1. **設定 `window.MEDIA_ENDPOINT`**：指向 S3 media endpoint
   ```js
   window.MEDIA_ENDPOINT = `https://s3.${region}.amazonaws.com/${bucket}/${prefix}/media`
   ```

2. **動態載入 `wistia-s3.min.js`**：
   ```js
   const script = document.createElement('script')
   script.src = wistiaS3JSUrl  // 從 config 或 env 取得
   script.onload = () => {
     // Player 會自動掃描 .wistia_embed 元素並初始化
   }
   document.head.appendChild(script)
   ```

3. **在 Vue component 中渲染 player container**：
   ```vue
   <div class="wistia_responsive_padding">
     <div class="wistia_responsive_wrapper">
       <div :class="`wistia_embed wistia_async_${hashId} videoFoam=true playsinline=true`" 
            style="height:100%;width:100%">&nbsp;</div>
     </div>
   </div>
   ```

4. **處理 SPA 路由切換**：
   - 在 `onMounted` 載入 script
   - 在 `onUnmounted` 清理 player instance（避免記憶體洩漏）
   - 使用 `nextTick` 確保 DOM 更新後再初始化 player

---

## 3. 字幕對齊編輯 UI 設計

### 3.1 佈局

```
┌─────────────────────────────────────────────────────────────┐
│  Video Player (wistia-s3-player)                            │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                                                       │  │
│  │                    [Video]                            │  │
│  │                                                       │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  字幕編輯器                                                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ [新增字幕]  [儲存]  [取消]  [下載 VTT]                 │  │
│  ├───────────────────────────────────────────────────────┤  │
│  │ # │ Start    │ End      │ 文字          │ 操作        │  │
│  │───┼──────────┼──────────┼───────────────┼─────────────│  │
│  │ 1 │ 00:00.00 │ 00:03.50 │ 歡迎收看...   │ ▶ ⋮ 🗑     │  │
│  │ 2 │ 00:03.50 │ 00:07.20 │ 今天我們要... │ ▶ ⋮ 🗑     │  │
│  │ 3 │ 00:07.20 │ 00:12.00 │ 首先介紹...   │ ▶ ⋮ 🗑     │  │
│  │...│ ...      │ ...      │ ...           │ ...         │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 功能規格

#### 表格編輯模式

| 欄位 | 類型 | 說明 |
|------|------|------|
| # | 自動編號 | 字幕序號 |
| Start | 時間輸入 | 格式 `MM:SS.ms`，可直接輸入或從 player 當前時間填入 |
| End | 時間輸入 | 格式 `MM:SS.ms`，可直接輸入或從 player 當前時間填入 |
| 文字 | 文字輸入 | 字幕內容，支援多行（textarea） |
| 操作 | 按鈕組 | ▶ 播放此句、⋮ 更多選項、🗑 刪除 |

#### 互動功能

1. **點擊 ▶ 播放**：Player seek 到 start 時間並播放
2. **播放時高亮**：根據 player 的 `currentTime` 自動高亮當前字幕列
3. **快捷鍵**：
   - `Ctrl+S`：儲存
   - `Ctrl+Enter`：播放/暫停
   - 在 Start/End 輸入框按 `Ctrl+I`：填入 player 當前時間
4. **新增字幕**：在表格末尾新增空白列
5. **刪除字幕**：確認後刪除該列
6. **拖曳排序**：可拖曳調整列順序（選用）

#### 時間輸入優化

- 支援多種格式自動解析：`0:03`、`00:03.5`、`00:03:500`
- 點擊輸入框旁的「擷取」按鈕，填入 player 當前時間
- 時間衝突檢查：End 必須 > Start，且不能與相鄰字幕重疊

### 3.3 儲存流程

1. 點擊「儲存」→ 前端驗證資料
2. 呼叫 `PUT /index/{hash}/subtitles` API
3. 後端更新：
   - BoltDB `index` bucket
   - S3 `media/{hash}/subtitles.vtt`
   - S3 `media/{hash}/index-ai.json`
   - CloudFront 同步（如有設定）
4. 顯示成功/失敗 toast

---

## 4. 後端 API 設計

### 4.1 新增 Endpoint

```
PUT /index/{hash}/subtitles
```

**Request Body:**
```json
{
  "subtitles": [
    { "start": 0.0, "end": 3.5, "text": "歡迎收看..." },
    { "start": 3.5, "end": 7.2, "text": "今天我們要..." }
  ]
}
```

**Response:**
```json
{
  "status": true,
  "data": {
    "hashId": "0axoa4ti38",
    "updatedAt": "2026-06-19T12:00:00Z",
    "subtitleCount": 2
  }
}
```

### 4.2 實作邏輯

```go
func (s *HTTPService) UpdateSubtitles(w http.ResponseWriter, r *http.Request) {
    // 1. 解析 request body
    // 2. 從 BoltDB 讀取現有 index
    // 3. 更新 subtitles 欄位
    // 4. 重新產生 VTT 內容
    // 5. 上傳到 S3 (subtitles.vtt + index-ai.json)
    // 6. 更新 BoltDB
    // 7. CloudFront invalidate (如有)
    // 8. 回傳結果
}
```

---

## 5. 影響範圍

### 5.1 Frontend (web/src/ui/)

| 檔案 | 動作 | 說明 |
|------|------|------|
| `views/VideoDetail.vue` | 修改 | 加入 player + subtitle editor |
| `components/VideoPlayer.vue` | 新增 | 封裝 wistia-s3-player 的 Vue component |
| `components/SubtitleEditor.vue` | 新增 | 字幕表格編輯器 |
| `api/index.js` | 修改 | 新增 `saveSubtitles()` function |

### 5.2 Backend (pkg/)

| 檔案 | 動作 | 說明 |
|------|------|------|
| `handler_index.go` | 修改 | 新增 `UpdateSubtitles` handler |
| `http.go` | 修改 | 註冊 `PUT /index/{hash}/subtitles` 路由 |

### 5.3 S3 Paths

既有路徑，內容會被更新：
```
{prefix}/media/{hashId}/subtitles.vtt
{prefix}/media/{hashId}/index-ai.json
{prefix}/cloudfront/media/{hashId}/subtitles.vtt    (如有 CloudFront)
{prefix}/cloudfront/media/{hashId}/index-ai.json    (如有 CloudFront)
```

---

## 6. 實作步驟

### Phase 1: Player 嵌入

| # | 任務 | 預估時間 |
|---|------|----------|
| 1.1 | 建立 `VideoPlayer.vue` component | 2h |
| 1.2 | 處理 script 動態載入與清理 | 1h |
| 1.3 | 整合到 `VideoDetail.vue` | 1h |
| 1.4 | 測試 player 播放功能 | 1h |

### Phase 2: 字幕編輯器

| # | 任務 | 預估時間 |
|---|------|----------|
| 2.1 | 建立 `SubtitleEditor.vue` 表格 UI | 3h |
| 2.2 | 實作時間輸入與驗證 | 2h |
| 2.3 | 實作 player 互動（seek、高亮） | 2h |
| 2.4 | 新增/刪除字幕功能 | 1h |

### Phase 3: 後端 API

| # | 任務 | 預估時間 |
|---|------|----------|
| 3.1 | 實作 `UpdateSubtitles` handler | 2h |
| 3.2 | 註冊路由 | 0.5h |
| 3.3 | S3 + BoltDB 更新邏輯 | 1h |
| 3.4 | CloudFront invalidate | 0.5h |

### Phase 4: 整合與測試

| # | 任務 | 預估時間 |
|---|------|----------|
| 4.1 | 前端 API 串接 | 1h |
| 4.2 | 端到端測試 | 2h |
| 4.3 | Bug fixes | 2h |

**總預估時間：約 18 小時**

---

## 7. 技術決策記錄

### 7.1 為什麼不直接用 HTML5 `<video>`？

| 方案 | 優點 | 缺點 |
|------|------|------|
| HTML5 `<video>` | 輕量、無依賴 | 需自實作控制列、畫質切換、字幕同步 |
| wistia-s3-player | 功能完整、與現有系統一致 | 需動態載入 script、與 Vue SPA 整合較複雜 |

**決定**：使用 `wistia-s3-player`，因為：
1. 與現有 demo page 一致，使用者體驗統一
2. 已實作畫質切換、字幕、章節等功能
3. 減少重複實作

### 7.2 為什麼用表格編輯而非時間軸拖曳？

| 方案 | 優點 | 缺點 |
|------|------|------|
| 表格編輯 | 精確、易實作、適合大量文字編輯 | 不夠直觀 |
| 時間軸拖曳 | 直觀 | 實作複雜、小螢幕操作困難 |
| 波形圖 | 最直觀 | 需額外處理音訊、實作最複雜 |

**決定**：表格編輯模式，因為：
1. 字幕文字需要精確編輯
2. 時間可透過「擷取當前時間」功能快速填入
3. 實作成本較低

---

## 8. 開放問題

1. **Player Script URL**：`wistia-s3.min.js` 的 URL 從哪裡取得？
   - 選項 A：從環境變數設定
   - 選項 B：從 S3 config 動態組合
   - 選項 C：硬編碼（不建議）

2. **字幕合併/分割**：是否需要支援？
   - 合併：選取多列 → 合併為一句
   - 分割：在播放中暫停 → 分割當前字幕
   - 建議：Phase 1 先不實作，視需求再加

3. **版本控制**：字幕編輯歷史是否需要記錄？
   - 選項 A：不記錄，直接覆蓋
   - 選項 B：記錄在 BoltDB 另一個 bucket
   - 建議：Phase 1 不記錄

4. **離線編輯**：是否需要支援？
   - 建議：Phase 1 不支援，必須連網儲存

---

## 9. 驗收標準

### 9.1 Player 嵌入

- [ ] VideoDetail 頁面最上方顯示 player
- [ ] 可正常播放/暫停影片
- [ ] 可切換畫質（如有多種解析度）
- [ ] 字幕可開啟/關閉
- [ ] 頁面離開後 player 正確清理

### 9.2 字幕編輯器

- [ ] 顯示所有字幕列（start/end/text）
- [ ] 可編輯 start/end 時間
- [ ] 可編輯文字內容
- [ ] 點擊 ▶ 可跳轉播放
- [ ] 播放時自動高亮當前字幕
- [ ] 可新增/刪除字幕列
- [ ] 儲存後 S3 + BoltDB 更新
- [ ] 儲存後 player 字幕同步更新

### 9.3 後端 API

- [ ] `PUT /index/{hash}/subtitles` 可正常接收請求
- [ ] 更新 BoltDB index bucket
- [ ] 更新 S3 subtitles.vtt
- [ ] 更新 S3 index-ai.json
- [ ] CloudFront invalidate（如有設定）
- [ ] 錯誤處理與回應格式正確

---

## 10. 附錄

### 10.1 相關檔案路徑

- Player 模板：`web/src/demo.html`, `web/src/index.html`
- Player entry：`web/src/main.js`
- Admin UI：`web/src/ui/`
- 後端 handler：`pkg/handler_index.go`
- S3 上傳：`pkg/storage_s3.go`
- BoltDB：`pkg/db.go`

### 10.2 參考資源

- Demo page：https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/0axoa4ti38/demo.html
- video.js 文件：https://videojs.com/
- Vue 3 文件：https://vuejs.org/

---

## 11. 實作完成記錄

**完成日期**: 2026-06-19

### 已完成的任務

| # | 任務 | 狀態 | 說明 |
|---|------|------|------|
| 1 | 建立 `VideoPlayer.vue` 元件 | ✅ | 動態載入 wistia-s3.min.js，暴露 seekTo/getCurrentTime |
| 2 | 建立 `SubtitleEditor.vue` 元件 | ✅ | 表格編輯、時間擷取、播放互動、VTT 下載 |
| 3 | 修改 `VideoDetail.vue` | ✅ | Player 在最上方，字幕編輯器在 AI 索引下方 |
| 4 | 修改 `api/index.js` | ✅ | 新增 `saveSubtitles()` |
| 5 | 後端 `UpdateSubtitles` handler | ✅ | 更新 S3 + BoltDB + CloudFront |
| 6 | 後端路由註冊 | ✅ | `PUT /index/{hash}/subtitles` |
| 7 | 編譯驗證 | ✅ | docker build + yarn build 通過 |

### 修改的檔案

**後端:**
- `pkg/handler_index.go` - 新增 `UpdateSubtitles` handler
- `pkg/http.go` - 註冊新路由

**前端:**
- `web/src/ui/api/index.js` - 新增 `saveSubtitles()`
- `web/src/ui/components/VideoPlayer.vue` - 新增
- `web/src/ui/components/SubtitleEditor.vue` - 新增
- `web/src/ui/views/VideoDetail.vue` - 整合 player + editor
