import { mkdirSync, readFileSync, writeFileSync, unlinkSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const root = fileURLToPath(new URL('../', import.meta.url))
const output = resolve(root, 'frontend/src/assets/file-icons')
mkdirSync(output, { recursive: true })
const manifestPath = resolve(output, 'manifest.json')
let previous = []
try { previous = JSON.parse(readFileSync(manifestPath, 'utf8')).icons } catch (error) { if (error.code !== 'ENOENT') throw error }

const escape = value => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;')
const letter = (value, size = 9) => `<text x="12" y="15.5" fill="currentColor" stroke="none" font-family="Arial,Helvetica,sans-serif" font-size="${size}" font-weight="600" text-anchor="middle">${escape(value)}</text>`
// Symbols use the same grid as the approved ten-icon preview.
const marks = {
  blank: '',
  unknown: letter('?', 12),
  none: '<path d="M8 12h8"/>',
  text: '<path d="M7 8h10M7 12h10M7 16h6"/>',
  log: '<path d="M7 7.5h7M7 11.5h5M7 15.5h3"/><circle cx="15" cy="15" r="3"/><path d="M15 13.5V15l1 1"/>',
  word: '<path d="m6.5 8 2 8 3.5-6 3.5 6 2-8" stroke-width="1.65"/>',
  sheet: '<rect x="6.5" y="6.5" width="11" height="11" rx="1"/><path d="M6.5 10h11M6.5 13.8h11M10.5 6.5v11"/>',
  csv: '<path d="M7 7h10v7H7zM7 10.5h10M10.5 7v7M14 16.5l-1 2"/>',
  numbers: '<path d="M7 17V12M12 17V7M17 17v-7" stroke-width="2.5"/>',
  slides: '<rect x="6.5" y="6.5" width="11" height="8" rx="1"/><path d="M12 14.5v3M9.5 17.5h5M9 9.5h5.5M9 11.5h3.5"/>',
  keynote: '<path d="M6.5 7h11v6h-11zM12 13v5M8 18h8"/><path d="m10 11 2-2 2 2"/>',
  book: '<path d="M12 8c-1.7-1.1-3.5-1.2-5.5-.5v9c2-.7 3.8-.6 5.5.5m0-9c1.7-1.1 3.5-1.2 5.5-.5v9c-2-.7-3.8-.6-5.5.5z"/>',
  pen: '<path d="m7 16 .8-3.5L15 5.3l3 3-7.2 7.2zM13.5 7l3 3M7 19h10"/>',
  image: '<circle cx="9" cy="8" r="1.1" fill="currentColor" stroke="none"/><path d="m6 16 4.5-5 3.2 3 2.3-2 2 4"/>',
  video: '<path d="m10 9.5 6 4-6 4Z" fill="currentColor" stroke="none"/>',
  audio: '<path d="M12.5 16V9c.8 1.6 3 1.9 3 4" stroke-width="1.35"/><ellipse cx="10.7" cy="16.3" rx="1.8" ry="1.3" fill="currentColor" stroke="none"/>',
  captions: '<rect x="6" y="7" width="12" height="10" rx="1.5"/><path d="M8.5 10h2M13.5 10h2M8.5 14h7"/>',
  playlist: '<path d="M6.5 8h7M6.5 12H11M6.5 16H11"/><path d="m14 11 4 3-4 3z" fill="currentColor" stroke="none"/>',
  zip: '<path d="M10.5 6.5H12M12 8.5h1.5M10.5 10.5H12M12 12.5h1.5"/><rect x="10.5" y="15" width="3" height="3" rx=".7"/>',
  rar: '<rect x="6.5" y="6" width="11" height="3.5" rx=".7"/><rect x="6.5" y="10.3" width="11" height="3.5" rx=".7"/><rect x="6.5" y="14.6" width="11" height="3.5" rx=".7"/><path d="M12 6v12"/>',
  compress: '<path d="M8 6v4m-2-2 2 2 2-2M16 18v-4m-2 2 2-2 2 2M7 14h3M14 10h3"/>',
  pdf: '<path d="M9 15.5c2.3-3.2 4.3-7.8 3.1-9.2-.9-1-1.6.1-1.2 2 .7 3.4 3.6 6.6 6 7 1.8.3 1.9-1.2.2-1.5-3.6-.7-8.9.7-10.2 2.3-1.1 1.4.8 2.2 2.1-.6Z" stroke-width="1.3"/>',
  code: '<path d="m8.5 8-3.5 4 3.5 4m7-8 3.5 4-3.5 4M13.5 7l-3 10"/>',
  braces: '<path d="M9.5 6.5H8v4L6.5 12 8 13.5v4h1.5M14.5 6.5H16v4l1.5 1.5-1.5 1.5v4h-1.5"/>',
  script: '<path d="m6.5 8.5 4 3.5-4 3.5M12.5 16h5"/>',
  settings: '<path d="M6 8h4M14 8h4M6 16h8M17 16h1"/><circle cx="12" cy="8" r="2"/><circle cx="15.5" cy="16" r="1.8"/>',
  yaml: '<path d="M6.5 7h7M9.5 12h8M9.5 17h8"/><circle cx="6.5" cy="12" r=".6" fill="currentColor"/><circle cx="6.5" cy="17" r=".6" fill="currentColor"/>',
  database: '<ellipse cx="12" cy="7.5" rx="5" ry="2"/><path d="M7 7.5v9c0 2.7 10 2.7 10 0v-9M7 12c0 2.7 10 2.7 10 0"/>',
  font: letter('Aa'),
  package: '<path d="m6.5 8.5 5.5-2.5 5.5 2.5v7L12 18l-5.5-2.5zm0 0L12 11l5.5-2.5M12 11v7M9.3 7.2l5.5 2.5"/>',
  executable: '<rect x="6" y="6.5" width="12" height="11" rx="1.5"/><path d="m10 9 4 3-4 3z" fill="currentColor" stroke="none"/>',
  disk: '<circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="1.5"/><path d="m15 8 1 1"/>',
  chip: '<rect x="8" y="8" width="8" height="8" rx="1"/><path d="M10 6v2M14 6v2M10 16v2M14 16v2M6 10h2M6 14h2M16 10h2M16 14h2"/>',
  backup: '<path d="M7 8a6 6 0 1 1-1 7M7 5v4H3M12 9v4l2 1"/>',
  clock: '<circle cx="12" cy="12" r="5.5"/><path d="M12 8.5v4l2.5 1.5"/>',
  cube: '<path d="m6.5 8.5 5.5-3 5.5 3v7l-5.5 3-5.5-3zm0 0 5.5 3 5.5-3M12 11.5v7"/>',
  cad: '<path d="m6 16 10-10 3 3L9 19zm4-4 2 2m1-5 2 2"/>',
  link: '<path d="m9.5 9.5 2-2a3 3 0 0 1 4.3 4.3l-2 2m-3.6-3.6-2 2a3 3 0 0 0 4.3 4.3l2-2M10 14l4-4"/>',
  download: '<path d="M12 6v9m-3-3 3 3 3-3M6.5 16v2h11v-2"/>',
  certificate: '<path d="m12 5.5 5.5 2v5c0 3-3 5-5.5 6-2.5-1-5.5-3-5.5-6v-5zM9 12l2 2 4-4"/>',
  key: '<circle cx="9" cy="9" r="3"/><path d="m11.3 11.3 6 6m-1.8-4.2L14 14.5m3-.5-1.5 1.5"/>',
  branch: '<circle cx="8" cy="7" r="1.7"/><circle cx="8" cy="17" r="1.7"/><circle cx="16" cy="7" r="1.7"/><path d="M8 8.7v6.6M16 8.7c0 4-8 1.5-8 5"/>',
  docker: '<path d="M5.5 12.5h13c-.5 4-3 5.5-7 5.5-3.5 0-5.5-2-6-5.5Z"/><path d="M8 12V9h3v3m0-3V6h3v3m0 3V9h3v3"/>',
  tool: '<path d="m7 17 6.5-6.5m-1-4.5a4 4 0 0 0 5.5 5.5l-3-1 .5-2.5z"/><circle cx="7" cy="17" r="1"/>',
  diamond: '<path d="m6 9 3-3h6l3 3-6 9zm0 0h12M9 6l3 12 3-12"/>',
  coffee: '<path d="M7 10h8v4a3 3 0 0 1-3 3h-2a3 3 0 0 1-3-3zm8 0h1.5a2 2 0 0 1 0 4H15M6 19h11M9 5v2M12 4v3"/>',
  atom: '<ellipse cx="12" cy="12" rx="7" ry="2.5"/><ellipse cx="12" cy="12" rx="7" ry="2.5" transform="rotate(60 12 12)"/><ellipse cx="12" cy="12" rx="7" ry="2.5" transform="rotate(120 12 12)"/><circle cx="12" cy="12" r=".8" fill="currentColor" stroke="none"/>',
  vue: '<path d="m6 7 6 11 6-11M10 7l2 4 2-4"/>',
  figma: '<rect x="8" y="5.5" width="4" height="4" rx="2"/><rect x="12" y="5.5" width="4" height="4" rx="2"/><rect x="8" y="9.5" width="4" height="4" rx="2"/><circle cx="14" cy="11.5" r="2"/><path d="M8 15.5a2 2 0 1 0 4 0v-2H10a2 2 0 0 0-2 2Z"/>',
  css: '<path d="m10 6-2 12M16 6l-2 12M6.5 10h11M6 14h11"/>',
  moon: '<path d="M15 6a6 6 0 1 0 3 9 6 6 0 0 1-6-8.5Z"/><circle cx="17" cy="8" r=".8" fill="currentColor"/>',
}

const icons = []
const aliases = {}
function tint(color, ratio) {
  return '#' + [1, 3, 5].map(i => Math.round(parseInt(color.slice(i, i + 2), 16) * ratio + 255 * (1 - ratio)).toString(16).padStart(2, '0')).join('').toUpperCase()
}
function add(id, category, mark, color, formats = '', label = id.toUpperCase(), names = []) {
  const extensions = formats ? formats.split(' ') : []
  const description = label
  icons.push({ id, category, color, label, description, extensions, names })
  const glyph = marks[mark] ?? letter(mark, mark.length > 2 ? 7 : 9)
  const transform = ['video', 'audio'].includes(id) ? '' : ' transform="translate(3 4.5) scale(.75)"'
  writeFileSync(resolve(output, `${id}.svg`), `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" role="img" aria-label="${escape(label)}">
  <title>${escape(label)}</title>
  <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" fill="${color}" fill-opacity=".035" stroke="${tint(color, .65)}" stroke-width=".9" stroke-linejoin="round"/>
  <path d="M14 2v4.5A1.5 1.5 0 0 0 15.5 8H20" stroke="${tint(color, .65)}" stroke-width=".9" stroke-linejoin="round"/>
  <g${transform} color="${tint(color, .9)}" stroke="currentColor" stroke-width="1.45" stroke-linecap="round" stroke-linejoin="round">${glyph}</g>
</svg>
`)
}
const neutral = '#718096', text = '#526782', blue = '#3265BE', green = '#25815C', orange = '#C05A31', red = '#BD4554', purple = '#8D4FB3', archive = '#A37220', code = '#367CA0'
add('file', '通用文件', 'blank', neutral, '', '普通文件')
add('unknown', '通用文件', 'unknown', neutral, '', '未知格式')
add('no-extension', '通用文件', 'none', neutral, '', '无扩展名文件')
add('txt', '文本与标记', 'text', text, 'txt', 'TXT')
add('markdown', '文本与标记', 'MD', text, 'md markdown', 'Markdown')
add('log', '文本与标记', 'log', text, 'log', '日志')
add('rtf', '文本与标记', 'pen', text, 'rtf', 'RTF')
add('tex', '文本与标记', 'TeX', text, 'tex', 'LaTeX')
add('word', 'Office 文档', 'word', blue, 'doc docx docm dot dotx', 'Word')
add('excel', 'Office 文档', 'sheet', green, 'xls xlsx xlsm xlsb xlt xltx', 'Excel')
add('powerpoint', 'Office 文档', 'slides', orange, 'ppt pptx pptm pps ppsx potx', 'PowerPoint')
add('pdf', 'PDF 与电子书', 'pdf', red, 'pdf', 'PDF')
add('ebook', 'PDF 与电子书', 'book', red, 'epub mobi azw azw3', '电子书')
add('odt', '其他办公文档', 'text', blue, 'odt', 'OpenDocument 文档')
add('pages', '其他办公文档', 'pen', blue, 'pages', 'Pages')
add('csv', '其他办公文档', 'csv', green, 'csv', 'CSV')
add('tsv', '其他办公文档', 'sheet', green, 'tsv', 'TSV')
add('ods', '其他办公文档', 'sheet', green, 'ods', 'OpenDocument 表格')
add('numbers', '其他办公文档', 'numbers', green, 'numbers', 'Numbers')
add('odp', '其他办公文档', 'slides', orange, 'odp', 'OpenDocument 演示')
add('keynote', '其他办公文档', 'keynote', orange, 'key', 'Keynote')
add('image', '图片', 'image', purple, 'jpg jpeg jpe jfif png gif webp avif bmp ico icns tif tiff heic heif svg raw dng cr2 cr3 nef arw', '图片')
add('audio', '音频', 'audio', '#B14579', 'mp3 wav flac aac m4a ogg wma aiff aif opus mid midi amr ape alac', '音频')
add('video', '视频', 'video', '#6C54AE', 'mp4 mov mkv avi webm wmv flv m4v mpeg mpg m2ts mts m2v 3gp vob ogv', '视频')
add('subtitle', '字幕与播放列表', 'captions', '#77599C', 'srt ass ssa vtt lrc sub', '字幕')
add('playlist', '字幕与播放列表', 'playlist', '#77599C', 'm3u m3u8 pls', '播放列表')
add('photoshop', '设计源文件', 'Ps', '#476E9F', 'psd psb', 'Photoshop')
add('illustrator', '设计源文件', 'Ai', '#AA7732', 'ai', 'Illustrator')
add('eps', '设计源文件', 'pen', '#A34383', 'eps', 'EPS 矢量')
add('sketch', '设计源文件', 'diamond', '#B18B35', 'sketch', 'Sketch')
add('figma', '设计源文件', 'figma', '#A34383', 'fig', 'Figma')
add('xd', '设计源文件', 'Xd', '#A34383', 'xd', 'Adobe XD')
add('affinity-design', '设计源文件', 'pen', '#805E91', 'afdesign', 'Affinity Designer')
add('affinity-photo', '设计源文件', 'image', '#805E91', 'afphoto', 'Affinity Photo')
add('zip', '压缩与归档', 'zip', archive, 'zip', 'ZIP')
add('rar', '压缩与归档', 'rar', archive, 'rar', 'RAR')
add('7z', '压缩与归档', '7z', archive, '7z', '7-Zip')
add('tar', '压缩与归档', 'package', archive, 'tar tgz', 'TAR')
add('compressed', '压缩与归档', 'compress', archive, 'gz bz2 xz zst zstd tbz2 txz', '压缩文件')
add('html', '网页与前端', 'code', code, 'html htm', 'HTML')
add('css', '网页与前端', 'css', code, 'css', 'CSS')
add('scss', '网页与前端', '$', '#A96191', 'scss sass', 'Sass / SCSS')
add('less', '网页与前端', 'braces', code, 'less', 'Less')
add('js', '网页与前端', 'JS', '#A1832F', 'js mjs cjs', 'JavaScript')
add('ts', '网页与前端', 'TS', blue, 'ts cts', 'TypeScript')
add('react', '网页与前端', 'atom', code, 'jsx tsx', 'React')
add('vue', '网页与前端', 'vue', green, 'vue', 'Vue')
add('svelte', '网页与前端', 'S', orange, 'svelte', 'Svelte')
for (const [id, mark, formats, label] of [
  ['python', 'Py', 'py pyw', 'Python'], ['java', 'coffee', 'java class jar', 'Java'],
  ['c', 'C', 'c h', 'C'], ['cpp', 'C+', 'cpp hpp cxx cc hxx', 'C++'],
  ['csharp', 'C#', 'cs', 'C#'], ['go', 'Go', 'go', 'Go'], ['rust', 'R', 'rs', 'Rust'],
  ['php', 'PHP', 'php', 'PHP'], ['ruby', 'diamond', 'rb', 'Ruby'], ['swift', 'S', 'swift', 'Swift'],
  ['kotlin', 'K', 'kt kts', 'Kotlin'], ['dart', 'D', 'dart', 'Dart'], ['lua', 'moon', 'lua', 'Lua'], ['r', 'R', 'r', 'R'],
]) add(id, '编程语言', mark, '#42728E', formats, label)
add('shell', '脚本', 'script', '#4C775B', 'sh bash zsh bat cmd', '命令脚本')
add('powershell', '脚本', 'script', blue, 'ps1 psm1', 'PowerShell')
add('json', '数据与配置', 'braces', '#627183', 'json jsonl ndjson', 'JSON')
add('xml', '数据与配置', 'code', '#627183', 'xml', 'XML')
add('yaml', '数据与配置', 'yaml', '#627183', 'yaml yml', 'YAML')
add('config', '数据与配置', 'settings', '#627183', 'toml ini conf cfg config properties', '配置文件')
add('env', '数据与配置', 'settings', '#627183', 'env', '环境配置')
add('sql', '数据库', 'database', '#3C8590', 'sql', 'SQL')
add('database', '数据库', 'database', '#3C8590', 'db sqlite sqlite3 db3 mdb accdb', '数据库')
add('font', '字体', 'font', '#805E91', 'ttf otf woff woff2 eot', '字体')
add('executable', '安装与可执行文件', 'executable', '#4C6EAD', 'exe app appimage', '应用程序')
add('installer', '安装与可执行文件', 'package', '#4C6EAD', 'msi pkg deb rpm', '安装包')
add('android', '安装与可执行文件', 'package', green, 'apk aab', 'Android 安装包')
add('ios', '安装与可执行文件', 'package', '#67778E', 'ipa', 'iOS 安装包')
add('disk', '系统与磁盘', 'disk', '#67778E', 'iso img dmg', '磁盘映像')
add('library', '系统与磁盘', 'chip', '#67778E', 'dll so dylib sys', '系统与库文件')
add('backup', '系统与磁盘', 'backup', '#67778E', 'bak backup', '备份文件')
add('temporary', '系统与磁盘', 'clock', '#67778E', 'tmp temp', '临时文件')
add('model', '3D 与 CAD', 'cube', '#35867D', 'obj fbx stl gltf glb 3ds c4d', '3D 模型')
add('blender', '3D 与 CAD', 'cube', orange, 'blend', 'Blender')
add('cad', '3D 与 CAD', 'cad', '#35867D', 'dwg dxf step stp', 'CAD 图纸')
add('torrent', '种子与快捷方式', 'download', '#437F9B', 'torrent', 'BT 种子')
add('shortcut', '种子与快捷方式', 'link', '#437F9B', 'lnk url website webloc', '快捷方式')
add('certificate', '证书与密钥', 'certificate', '#96752A', 'pem crt cer p7b p7c', '证书')
add('private-key', '证书与密钥', 'key', '#96752A', 'p12 pfx', '密钥')
add('dockerfile', '特殊文件名', 'docker', code, '', 'Dockerfile', ['dockerfile'])
add('makefile', '特殊文件名', 'tool', '#4C775B', '', 'Makefile', ['makefile'])
add('license', '特殊文件名', 'certificate', '#96752A', '', '许可证', ['license', 'licence', 'copying'])
add('readme', '特殊文件名', 'book', blue, '', 'README', ['readme'])
add('gitignore', '特殊文件名', 'branch', '#B76141', '', 'Git 配置', ['.gitignore', '.gitattributes', '.gitmodules'])

// The .ts and .key defaults remain TypeScript and Keynote; components can override them.
aliases['video-ts'] = 'video'
writeFileSync(manifestPath, JSON.stringify({ icons, aliases }, null, 2) + '\n')
writeFileSync(resolve(output, 'folder.svg'), `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" role="img" aria-label="文件夹">
  <title>文件夹</title>
  <path d="M4 4h5l2 2h9a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z" fill="#E8B85A"/>
</svg>
`)
// Remove only obsolete files explicitly owned by the previous catalog.
const current = new Set(icons.map(icon => icon.id))
for (const { id } of previous) {
  if (/^[a-z0-9-]+$/.test(id) && !current.has(id)) {
    try { unlinkSync(resolve(output, `${id}.svg`)) } catch (error) { if (error.code !== 'ENOENT') throw error }
  }
}
console.log(`Generated ${icons.length + 1} SVG icons covering ${icons.reduce((sum, icon) => sum + icon.extensions.length, 0)} extensions.`)
