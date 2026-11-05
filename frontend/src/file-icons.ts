export { fileIconCatalog, fileIconId } from './file-icon-id'
import { fileIconAliases } from './file-icon-id'

const assets = import.meta.glob<string>('./assets/file-icons/*.svg', {
  eager: true, query: '?url', import: 'default',
})
export function fileIconUrl(id: string): string {
  const resolved = fileIconAliases[id] || id
  return assets[`./assets/file-icons/${resolved}.svg`] || assets['./assets/file-icons/unknown.svg']!
}
