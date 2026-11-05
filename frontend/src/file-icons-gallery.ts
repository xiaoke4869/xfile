import { fileIconCatalog, fileIconUrl } from './file-icons'

const icons = [
  { id: 'folder', category: '通用文件', label: '文件夹', description: '文件夹', extensions: [], names: [] },
  ...fileIconCatalog,
]
const search = document.querySelector<HTMLInputElement>('#search')!
const category = document.querySelector<HTMLSelectElement>('#category')!
const catalog = document.querySelector<HTMLDivElement>('#catalog')!
for (const name of new Set(icons.map(icon => icon.category))) category.add(new Option(name, name))

function render() {
  const query = search.value.trim().toLowerCase()
  const filtered = icons.filter(icon => (!category.value || icon.category === category.value) &&
    [icon.id, icon.label, icon.description, icon.category, ...icon.extensions, ...icon.names].join(' ').toLowerCase().includes(query))
  document.querySelector('#count')!.textContent = `${filtered.length} / ${icons.length} 个图标`
  catalog.replaceChildren()
  for (const name of new Set(filtered.map(icon => icon.category))) {
    const section = document.createElement('section')
    const heading = document.createElement('h2')
    const group = filtered.filter(icon => icon.category === name)
    heading.textContent = name
    const count = document.createElement('span')
    count.textContent = String(group.length)
    heading.append(count)
    const grid = document.createElement('div')
    grid.className = 'grid'
    for (const icon of group) {
      const link = document.createElement('a')
      link.className = 'icon'
      link.href = fileIconUrl(icon.id)
      link.download = `${icon.id}.svg`
      link.title = `下载 ${icon.description} SVG${icon.extensions.length ? ' · ' + icon.extensions.join(' / ') : ''}`
      const image = document.createElement('img')
      image.src = link.href
      image.alt = ''
      image.width = image.height = 24
      const label = document.createElement('small')
      label.textContent = icon.label
      link.append(image, label)
      const formats = document.createElement('span')
      formats.className = 'icon-formats'
      const supported = icon.extensions.length ? icon.extensions : icon.names
      formats.textContent = supported.slice(0, 3).join(' / ') + (supported.length > 3 ? ` +${supported.length - 3}` : '')
      link.append(formats)
      link.setAttribute('aria-label', link.title)
      grid.append(link)
    }
    section.append(heading, grid)
    catalog.append(section)
  }
  if (!filtered.length) {
    const empty = document.createElement('p')
    empty.className = 'empty'
    empty.textContent = '没有匹配的图标，请尝试其他格式或分类。'
    catalog.append(empty)
  }
}
search.addEventListener('input', render)
category.addEventListener('change', render)
document.querySelector<HTMLSelectElement>('#size')!.addEventListener('change', event => {
  document.documentElement.style.setProperty('--icon-size', `${(event.target as HTMLSelectElement).value}px`)
})
document.querySelector<HTMLButtonElement>('#theme')!.addEventListener('click', event => {
  const button = event.currentTarget as HTMLButtonElement
  const dark = document.documentElement.dataset.theme !== 'dark'
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
  button.setAttribute('aria-pressed', String(dark))
  button.textContent = dark ? '浅色背景' : '深色背景'
})
render()
