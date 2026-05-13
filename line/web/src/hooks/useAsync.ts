import { useState, useCallback, useRef, useEffect } from 'react'

interface AsyncState<T> {
  data: T | null
  loading: boolean
  error: Error | null
}

interface UseAsyncReturn<T> extends AsyncState<T> {
  execute: () => Promise<void>
  retry: () => Promise<void>
}

export function useAsync<T>(
  fn: () => Promise<T>,
  deps: unknown[] = [],
  immediate = true,
): UseAsyncReturn<T> {
  const [state, setState] = useState<AsyncState<T>>({
    data: null,
    loading: immediate,
    error: null,
  })
  const mountedRef = useRef(true)
  const retryCountRef = useRef(0)

  useEffect(() => {
    mountedRef.current = true
    return () => { mountedRef.current = false }
  }, [])

  const execute = useCallback(async () => {
    setState(s => ({ ...s, loading: true, error: null }))
    try {
      const data = await fn()
      if (mountedRef.current) {
        setState({ data, loading: false, error: null })
        retryCountRef.current = 0
      }
    } catch (err) {
      if (mountedRef.current) {
        setState(s => ({ ...s, loading: false, error: err as Error }))
      }
    }
  }, deps)

  const retry = useCallback(async () => {
    retryCountRef.current++
    const delay = Math.min(1000 * 2 ** retryCountRef.current, 10000)
    await new Promise(r => setTimeout(r, delay))
    return execute()
  }, [execute])

  useEffect(() => {
    if (immediate) execute()
  }, [execute, immediate])

  return { ...state, execute, retry }
}
