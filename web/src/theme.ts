// 界面主题（light/dark），持久化到 localStorage，通过自定义事件通知 App 重渲染
export type ThemeMode = 'light' | 'dark'

const THEME_KEY = 'fenghuo.theme'
const THEME_EVENT = 'fenghuo-theme-change'

export function getThemeMode(): ThemeMode {
  return localStorage.getItem(THEME_KEY) === 'dark' ? 'dark' : 'light'
}

export function setThemeMode(mode: ThemeMode): void {
  localStorage.setItem(THEME_KEY, mode)
  window.dispatchEvent(new Event(THEME_EVENT))
}

export function onThemeChange(listener: () => void): () => void {
  window.addEventListener(THEME_EVENT, listener)
  return () => window.removeEventListener(THEME_EVENT, listener)
}
