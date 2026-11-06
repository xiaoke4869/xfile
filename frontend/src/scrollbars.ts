import type { ObjectDirective } from 'vue'
import { OverlayScrollbars } from 'overlayscrollbars'
import 'overlayscrollbars/overlayscrollbars.css'
import './scrollbars.css'

type Axis = 'x' | 'y' | undefined
type ScrollbarBinding = Axis | false
const instances = new WeakMap<HTMLElement, { instance: OverlayScrollbars; axis: Axis }>()

function mount(element: HTMLElement, axis: Axis) {
 // Keep the original viewport: selection, drag scrolling and dialog refs use it directly.
 const instance = OverlayScrollbars({ target: element, elements: { viewport: element } }, {
  overflow: { x: axis === 'y' ? 'hidden' : 'scroll', y: axis === 'x' ? 'hidden' : 'scroll' },
  scrollbars: { theme: 'os-theme-xfile', autoHide: 'move', autoHideDelay: 800, autoHideSuspend: false },
 })
 for (const { scrollbar } of [instance.elements().scrollbarHorizontal, instance.elements().scrollbarVertical]) {
  scrollbar.setAttribute('aria-hidden', 'true')
  scrollbar.addEventListener('pointerdown', event => event.stopPropagation())
 }
 instances.set(element, { instance, axis })
}

export const vScrollbars: ObjectDirective<HTMLElement, ScrollbarBinding> = {
 mounted(element, { value }) { if (value !== false) mount(element, value) },
 updated(element, { value }) {
  const current = instances.get(element)
  if (current && (value === false || current.axis !== value)) {
   current.instance.destroy()
   instances.delete(element)
  }
  if (value === false) return
  if (!instances.has(element)) mount(element, value)
  const instance = instances.get(element)!.instance
  // Vue may replace a text-only node's contents; restore the library-owned scrollbars.
  for (const { scrollbar } of [instance.elements().scrollbarHorizontal, instance.elements().scrollbarVertical]) {
   if (scrollbar.parentElement !== element) element.append(scrollbar)
  }
  instance.update()
 },
 beforeUnmount(element) { instances.get(element)?.instance.destroy(); instances.delete(element) },
}
