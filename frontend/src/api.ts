import { Call } from '@wailsio/runtime'
export const native=!!((window as any).chrome?.webview?.postMessage||(window as any).webkit?.messageHandlers?.external?.postMessage)
export async function api<T>(method:string,...args:unknown[]):Promise<T>{
 if(!native){throw new Error('请在 XFile 桌面窗口中使用本地文件功能。浏览器预览不具备本地权限。')}
 return await Call.ByName(`main.FileService.${method}`,...args) as T
}
