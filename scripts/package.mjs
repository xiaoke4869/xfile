import { copyFileSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const [version, target = process.platform === 'darwin' ? 'all' : 'windows', ...extra] = process.argv.slice(2)

function main() {
  if (extra.length || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version ?? '')) {
    throw new Error('用法：node scripts/package.mjs <版本号，例如 0.0.2> [all|macos|windows]')
  }
  // CFBundleVersion permits four digits for major and two each for minor/patch.
  if (version.split('.').some((part, i) => Number(part) > [9999, 99, 99][i])) {
    throw new Error('为兼容 macOS，主版本不超过 9999，次版本和修订号不超过 99。')
  }
  if (!['all', 'macos', 'windows'].includes(target)) throw new Error('打包目标须为 all、macos 或 windows。')
  if (target !== 'windows' && process.platform !== 'darwin') throw new Error('macOS DMG 须在 macOS 上打包；当前系统请指定 windows。')

  // Prepare every change before writing, so a missing metadata field fails early.
  const updates = new Map()
  function update(path, pattern, replacement, expected) {
    const text = updates.get(path) ?? readFileSync(join(root, path), 'utf8')
    let count = 0
    const next = text.replace(pattern, (...args) => {
      count++
      return replacement(...args)
    })
    if (count !== expected) throw new Error(`${path}：版本字段数量异常，预期 ${expected}，实际 ${count}。`)
    updates.set(path, next)
  }
  const quotedVersion = (_match, before, after) => `${before}${version}${after}`
  update('build/config.yml', /^(  version:\s*")[^"]+(".*)$/gm, quotedVersion, 1)
  for (const path of ['build/darwin/Info.plist', 'build/darwin/Info.dev.plist']) {
    update(path, /(<key>CFBundle(?:ShortVersionString|Version)<\/key>\s*<string>)[^<]+(<\/string>)/g, quotedVersion, 2)
  }
  update('build/windows/info.json', /("(?:file_version|ProductVersion)"\s*:\s*")[^"]+(")/g, quotedVersion, 2)
  update('build/windows/nsis/wails_tools.nsh', /(!define INFO_PRODUCTVERSION ")[^"]+(")/g, quotedVersion, 1)
  for (const path of ['build/windows/msix/app_manifest.xml', 'build/windows/msix/template.xml']) {
    update(path, /(\bVersion=")[^"]+(")/g, (_match, before, after) => `${before}${version}.0${after}`, 1)
  }
  update('frontend/package.json', /("version"\s*:\s*")[^"]+(")/g, quotedVersion, 1)
  const lockPath = 'frontend/package-lock.json'
  const lockText = readFileSync(join(root, lockPath), 'utf8')
  const lock = JSON.parse(lockText)
  if (typeof lock.version !== 'string' || typeof lock.packages?.['']?.version !== 'string') {
    throw new Error(`${lockPath}：缺少根包版本。`)
  }
  lock.version = lock.packages[''].version = version
  updates.set(lockPath, JSON.stringify(lock, null, lockText.match(/\n([ \t]+)"/)?.[1] ?? 2) + '\n')
  for (const [path, text] of updates) writeFileSync(join(root, path), text)
  console.log(`应用版本：${version}`)

  function run(...args) {
    const command = process.platform === 'win32' ? (process.env.ComSpec || 'cmd.exe') : 'sh'
    const prefix = process.platform === 'win32' ? ['/d', '/c', 'scripts\\xfile.cmd'] : ['scripts/xfile.sh']
    const result = spawnSync(command, [...prefix, ...args], { cwd: root, stdio: 'inherit' })
    if (result.error) throw result.error
    if (result.status !== 0) throw new Error(`打包失败：${args.join(' ')}（${result.signal || result.status}）`)
  }
  function save(source, destination) {
    copyFileSync(join(root, 'bin', source), join(root, 'bin', destination))
    console.log(`已生成：bin/${destination}`)
  }
  if (target === 'all' || target === 'macos') {
    run('task', 'darwin:package:universal')
    run('task', 'darwin:create:dmg')
    save('XFile.dmg', `XFile-${version}-macos-universal.dmg`)
  }
  if (target === 'all' || target === 'windows') {
    run('package', 'GOOS=windows', 'GOARCH=amd64')
    save('XFile-amd64-installer.exe', `XFile-${version}-windows-x64-setup.exe`)
  }
}

try {
  main()
} catch (error) {
  console.error(error.message)
  process.exitCode = 1
}
