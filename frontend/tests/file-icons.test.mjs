import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { fileIconCatalog, fileIconId } from '../src/file-icon-id.ts'

test('every supported format resolves to its assigned SVG, including uppercase extensions', () => {
  const ids = new Set()
  for (const icon of fileIconCatalog) {
    assert.ok(!ids.has(icon.id), `Duplicate icon ${icon.id}`)
    ids.add(icon.id)
    assert.ok(existsSync(new URL(`../src/assets/file-icons/${icon.id}.svg`, import.meta.url)))
    for (const extension of icon.extensions) {
      assert.equal(fileIconId(`测试文件.${extension.toUpperCase()}`), icon.id)
    }
    for (const name of icon.names) assert.equal(fileIconId(name.toUpperCase()), icon.id)
  }
})

test('filename matching handles paths, dotfiles, compound extensions, aliases and fallbacks', () => {
  const cases = [
    ['目录.PDF', 'folder', true],
    ['/home/user/report.final.PDF', 'pdf'],
    ['C:\\Users\\test\\photo.JPEG', 'image'],
    ['bundle.tar.gz', 'tar'],
    ['archive.tar.bz2', 'compressed'],
    ['image.TIF', 'image'],
    ['photo.heif', 'image'],
    ['module.mjs', 'js'],
    ['.env', 'env'],
    ['.env.production.local', 'env'],
    ['.gitignore', 'gitignore'],
    ['.unknown-setting', 'unknown'],
    ['README.zh-CN.md', 'readme'],
    ['LICENSE.txt', 'license'],
    ['Dockerfile.production', 'dockerfile'],
    ['Makefile.local', 'makefile'],
    ['draft.something-new', 'unknown'],
    ['notes.', 'unknown'],
    ['notes', 'no-extension'],
    ['中文文件', 'no-extension'],
    ['', 'file'],
  ]
  for (const [name, expected, directory] of cases) assert.equal(fileIconId(name, directory), expected, name)
})

test('ambiguous extensions have documented defaults and separate alternate assets', () => {
  assert.equal(fileIconId('app.ts'), 'ts')
  assert.equal(fileIconId('presentation.key'), 'keynote')
  assert.equal(fileIconId('clip.m2ts'), 'video')
  for (const id of ['video', 'private-key', 'folder']) {
    assert.ok(existsSync(new URL(`../src/assets/file-icons/${id}.svg`, import.meta.url)))
  }
})

test('media formats share one icon per category while Office and PDF remain distinct', () => {
  for (const extension of ['mp4', 'mov', 'mkv', 'avi', 'webm', 'mpeg', 'm2ts']) assert.equal(fileIconId(`clip.${extension}`), 'video')
  for (const extension of ['mp3', 'wav', 'flac', 'm4a', 'ogg', 'opus', 'mid']) assert.equal(fileIconId(`sound.${extension}`), 'audio')
  for (const extension of ['jpg', 'png', 'gif', 'webp', 'svg', 'raw', 'heic']) assert.equal(fileIconId(`picture.${extension}`), 'image')
  const office = [fileIconId('document.docx'), fileIconId('table.xlsx'), fileIconId('slides.pptx'), fileIconId('document.pdf')]
  assert.equal(new Set(office).size, 4)
  assert.deepEqual(office, ['word', 'excel', 'powerpoint', 'pdf'])
})
