import test from 'node:test'
import assert from 'node:assert/strict'
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const paths = [
  'build/config.yml', 'build/darwin/Info.plist', 'build/darwin/Info.dev.plist',
  'build/windows/info.json', 'build/windows/nsis/wails_tools.nsh',
  'build/windows/msix/app_manifest.xml', 'build/windows/msix/template.xml',
  'frontend/package.json', 'frontend/package-lock.json', 'scripts/package.mjs',
]

test('versioned packaging validates metadata, repeats builds and stops on failure', { skip: process.platform === 'win32' }, () => {
  const fixture = mkdtempSync(join(tmpdir(), 'xfile-package-'))
  try {
    for (const path of paths) {
      mkdirSync(dirname(join(fixture, path)), { recursive: true })
      cpSync(join(root, path), join(fixture, path))
    }
    writeFileSync(join(fixture, 'scripts/xfile.sh'), `#!/bin/sh
set -eu
echo "$*" >> calls
if [ "\${FAIL_BUILD:-}" = 1 ]; then exit 7; fi
mkdir -p bin
case "$*" in
  'task darwin:create:dmg') echo dmg > bin/XFile.dmg ;;
  'package GOOS=windows GOARCH=amd64') echo installer > bin/XFile-amd64-installer.exe ;;
esac
`)
    const run = (version, target = 'windows', env = {}) => spawnSync(process.execPath,
      [join(fixture, 'scripts/package.mjs'), version, target], { encoding: 'utf8', env: { ...process.env, ...env } })
    const config = () => readFileSync(join(fixture, 'build/config.yml'), 'utf8')
    const initial = config()
    const initialLock = JSON.parse(readFileSync(join(fixture, 'frontend/package-lock.json')))
    for (const version of ['v1.2.3', '1.2', '01.2.3', '1.2.3-beta', '1.100.0', '../1.2.3']) {
      assert.notEqual(run(version).status, 0)
      assert.equal(config(), initial)
    }
    assert.notEqual(run('1.2.3', 'invalid').status, 0)
    const infoPath = join(fixture, 'build/windows/info.json')
    const info = readFileSync(infoPath, 'utf8')
    writeFileSync(infoPath, '{}')
    assert.notEqual(run('1.2.3').status, 0)
    assert.equal(config(), initial, 'metadata mismatch must fail before any writes')
    writeFileSync(infoPath, info)

    const target = process.platform === 'darwin' ? 'all' : 'windows'
    const result = run('1.2.3', target)
    assert.equal(result.status, 0, result.stderr)
    assert.match(config(), /  version: "1\.2\.3"/)
    for (const path of ['build/darwin/Info.plist', 'build/darwin/Info.dev.plist']) {
      const plist = readFileSync(join(fixture, path), 'utf8')
      for (const key of ['CFBundleVersion', 'CFBundleShortVersionString']) {
        assert.match(plist, new RegExp(`<key>${key}</key>\\s*<string>1\\.2\\.3</string>`))
      }
    }
    const updatedInfo = JSON.parse(readFileSync(infoPath, 'utf8'))
    assert.equal(updatedInfo.fixed.file_version, '1.2.3')
    assert.equal(updatedInfo.info['0409'].ProductVersion, '1.2.3')
    assert.match(readFileSync(join(fixture, 'build/windows/nsis/wails_tools.nsh'), 'utf8'), /!define INFO_PRODUCTVERSION "1\.2\.3"/)
    for (const path of ['build/windows/msix/app_manifest.xml', 'build/windows/msix/template.xml']) {
      assert.match(readFileSync(join(fixture, path), 'utf8'), /Version="1\.2\.3\.0"/)
    }
    assert.equal(JSON.parse(readFileSync(join(fixture, 'frontend/package.json'))).version, '1.2.3')
    const lock = JSON.parse(readFileSync(join(fixture, 'frontend/package-lock.json')))
    assert.equal(lock.version, '1.2.3')
    assert.equal(lock.packages[''].version, '1.2.3')
    for (const [path, dependency] of Object.entries(initialLock.packages)) {
      if (path) assert.deepEqual(lock.packages[path], dependency, 'dependency versions must stay unchanged')
    }
    const installer = join(fixture, 'bin/XFile-1.2.3-windows-x64-setup.exe')
    assert.equal(readFileSync(installer, 'utf8'), 'installer\n')
    const calls = readFileSync(join(fixture, 'calls'), 'utf8').trim().split('\n')
    assert.deepEqual(calls, target === 'all'
      ? ['task darwin:package:universal', 'task darwin:create:dmg', 'package GOOS=windows GOARCH=amd64']
      : ['package GOOS=windows GOARCH=amd64'])
    if (target === 'all') assert.equal(readFileSync(join(fixture, 'bin/XFile-1.2.3-macos-universal.dmg'), 'utf8'), 'dmg\n')
    writeFileSync(installer, 'old build')
    assert.equal(run('1.2.3').status, 0)
    assert.equal(readFileSync(installer, 'utf8'), 'installer\n')
    assert.notEqual(run('1.2.4', 'windows', { FAIL_BUILD: '1' }).status, 0)
    assert.equal(existsSync(join(fixture, 'bin/XFile-1.2.4-windows-x64-setup.exe')), false)
    assert.equal(existsSync(installer), true)
  } finally {
    rmSync(fixture, { recursive: true, force: true })
  }
})
