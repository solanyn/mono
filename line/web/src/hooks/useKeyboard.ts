import { useEffect, useCallback } from 'react'

type KeyHandler = (e: KeyboardEvent) => void

interface KeyMap {
  [key: string]: KeyHandler
}

export function useKeyboard(keyMap: KeyMap, deps: unknown[] = []) {
  const handler = useCallback((e: KeyboardEvent) => {
    const key = [
      e.ctrlKey && 'ctrl',
      e.metaKey && 'meta',
      e.shiftKey && 'shift',
      e.altKey && 'alt',
      e.key.toLowerCase(),
    ].filter(Boolean).join('+')

    const fn = keyMap[key] || keyMap[e.key.toLowerCase()]
    if (fn) {
      e.preventDefault()
      fn(e)
    }
  }, deps)

  useEffect(() => {
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [handler])
}

export function useEscape(handler: () => void, deps: unknown[] = []) {
  useKeyboard({ escape: handler }, deps)
}
