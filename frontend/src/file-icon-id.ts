import manifest from './assets/file-icons/manifest.json' with { type: 'json' }

export const fileIconCatalog = manifest.icons
export const fileIconAliases: Record<string, string> = manifest.aliases
const extensions = new Map<string, string>(Object.entries(fileIconAliases))
const names = new Map<string, string>()
for (const icon of fileIconCatalog) {
  for (const extension of icon.extensions) extensions.set(extension, icon.id)
  for (const name of icon.names) names.set(name, icon.id)
}

export function fileIconId(filename: string, directory = false): string {
  if (directory) return 'folder'
  const name = filename.split(/[\\/]/).pop()?.toLowerCase() || ''
  if (!name) return 'file'
  const exact = names.get(name)
  if (exact) return exact
  // Keep named files recognizable when they have a suffix (.env.local, README.md).
  for (const prefix of ['.env', 'dockerfile', 'makefile', 'readme', 'license', 'licence']) {
    if (name.startsWith(prefix + '.')) return prefix === '.env' ? 'env' : names.get(prefix)!
  }
  if (name.endsWith('.tar.gz')) return 'tar'
  const dot = name.lastIndexOf('.')
  if (dot < 0) return 'no-extension'
  return extensions.get(name.slice(dot + 1)) || 'unknown'
}
