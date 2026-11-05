<p align="center">
  <img src="build/appicon.png" alt="XFile 图标" width="128" height="128">
</p>

# XFile

一个专注于批量整理文件与目录的桌面工具。

归并分散的目录、提取深层文件、导出目录结构、批量修改名称。适用于 macOS 和 Windows，文件处理在本地完成，无需部署服务。

[功能特性](#功能特性) · [技术栈](#技术栈) · [本地开发](#本地开发) · [构建打包](#构建打包)

## 功能特性

- **默认应用打开**：双击文件调用系统默认应用打开，双击文件夹进入目录；支持 macOS 和 Windows。
- **归并目录**：按目录名称、数字编号或自定义规则，将不同位置的目录归并到一起，保留内部结构，执行前预览归并结果。
- **提取文件**：将多层目录中的文件递归移动到同一个目标目录，支持多来源与重名处理。
- **文件结构导出**：将目录内容导出为 UTF-8 TXT，支持单层文件清单和多层目录树，方便整理资料、分享文件结构。
- **批量重命名**：支持文本替换、添加前后缀和按格式编号，保留文件扩展名，实时预览修改结果。

## 技术栈

| 技术 | 用途 |
| --- | --- |
| [Go](https://go.dev/) | 本地文件处理与任务执行 |
| [Wails 3](https://wails.io/) | 桌面应用框架与 Go / 前端通信 |
| [Vue 3](https://vuejs.org/) + [TypeScript](https://www.typescriptlang.org/) | 用户界面与交互 |
| [Vite](https://vite.dev/) | 前端开发与构建 |

## 本地开发

macOS 窗口使用原生红绿灯与隐藏标题栏，左侧栏透出系统玻璃材质，文件列表保持实色背景。macOS 26 及以上使用 Liquid Glass，旧系统回退为磨砂效果；Windows 沿用原生窗口边框。顶部空白区域可拖动窗口。

macOS 的开发及打包任务已包含 `private_mac_apis` 构建标签，用于启用 Wails 的透明 WebView。若直接运行 `go build`，也需添加 `-tags private_mac_apis`；浏览器预览不显示系统玻璃和红绿灯。

### 环境要求

- Go 1.26.10。
- Node.js 22.12+ 或 24+，以及 npm。
- macOS 需安装 Xcode Command Line Tools；Windows 需准备 C/C++ 编译环境和 WebView2 Runtime，详见 [Wails 环境配置](https://wails.io/)。

### 启动

```sh
git clone git@github.com:xiaoke4869/xfile.git
cd xfile

go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0

cd frontend
npm ci
cd ..

wails3 dev
```

确保 Go 的可执行文件目录已加入 `PATH`，可以直接运行 `wails3`。

### 检查

在项目根目录执行，先生成前端接口绑定并构建资源，再运行 Go 检查：

```sh
wails3 task common:build:frontend
go test -race ./...
go vet ./...
node --test scripts/package.test.mjs
```

## 构建打包

### 指定版本号打包（推荐）

在项目根目录执行，版本号由你指定，不会自动递增：

```sh
sh scripts/package.sh 0.0.2
```

macOS 默认打包 macOS 通用版和 Windows x64 两个平台，需安装 NSIS；Windows 默认只打包 Windows。也可以指定目标：

```sh
sh scripts/package.sh 0.0.2 macos
sh scripts/package.sh 0.0.2 windows
```

在 Windows 命令提示符中使用：

```bat
scripts\package.cmd 0.0.2
```

脚本会同步 `build/config.yml`、macOS 应用、Windows 应用及安装器、前端包信息中的版本，然后调用现有构建流程。版本号采用三个不带前导零的整数，不含 `v` 前缀或预发布后缀；为兼容 macOS，主版本不超过 9999，次版本和修订号不超过 99。

安装包输出为 `bin/XFile-0.0.2-macos-universal.dmg` 和 `bin/XFile-0.0.2-windows-x64-setup.exe`。同一版本可重复打包并覆盖同名文件，其他版本的文件保留。原有无版本号产物也会保留在 `bin/`。

版本配置的修改会保留在源码中，即使构建失败也不会回退；修复后可用同一版本号重试。发布时将这些修改与源码一起提交，并创建对应的 `v0.0.2` Git 标签。脚本只负责本地打包，GitHub Release 需另行发布。

### macOS

构建适用于 Intel 和 Apple Silicon 的通用应用及 DMG 安装镜像：

```sh
wails3 task darwin:package:universal
wails3 task darwin:create:dmg
```

### Windows

构建 x64 应用和安装包，安装包构建需安装 NSIS：

```sh
wails3 build GOOS=windows GOARCH=amd64
wails3 package GOOS=windows GOARCH=amd64
```

构建产物输出到 `bin/`。

### 应用图标

图标资源集中存放在 `build/`：`appicon.svg` 保留矢量源文件，`appicon.png` 是用于打包和 README 展示的 1024 × 1024 PNG。GitHub README 使用仓库内相对路径引用图片，克隆仓库或切换分支后仍可正常显示。

替换 PNG 后，正常构建会自动生成 macOS 的 `build/darwin/icons.icns` 和 Windows 的 `build/windows/icon.ico`，供应用和安装包使用。也可以单独重新生成：

```sh
wails3 task common:generate:icons
```

### 文件格式图标

文件图标位于 `frontend/src/assets/file-icons/`，共 93 个独立 SVG，支持 234 种扩展名和特殊文件名。统一采用 24 × 24 网格、浅色细描边文件轮廓和简洁类型符号；文件夹使用单色扁平图形，无外部字体或图片依赖。

视频（MP4/MOV/MKV 等）、音频（MP3/WAV/FLAC 等）、图片（JPG/PNG/WEBP/SVG/RAW 等）分别共用一个图标；视频使用播放三角，音频使用单音符，图片使用山景符号。Word、Excel、PowerPoint、PDF 分别使用 W、表格、演示屏幕和 PDF 符号，其他常见类型按格式或用途分配图标。完整映射保存在同目录的 `manifest.json`。

文件列表、预览树、已选文件、移动目录选择器和操作明细共用 `FileIcon.vue`。匹配忽略扩展名大小写，支持 Windows/macOS 路径、`.env.local`、`README.md`、`Dockerfile.production` 和 `.tar.gz` 等名称。扩展名不能区分内容时，`.ts` 默认使用 TypeScript、`.key` 默认使用 Keynote；可通过组件的 `format="video"` 或 `format="private-key"` 指定视频或密钥图标。旧的 `format="video-ts"` 继续映射到视频图标。

在 `frontend/` 下执行：

```sh
npm run dev
# 打开 http://127.0.0.1:5173/file-icons.html 查看整套图标
npm run icons:generate # 修改 scripts/generate-file-icons.mjs 后重新生成
npm run test:icons     # 检查格式匹配和图标完整性
npm run build
```

图标预览页支持搜索、分类筛选、24/32/48 px 尺寸、深浅背景与单个 SVG 下载，构建后保留为 `frontend/dist/file-icons.html`。
