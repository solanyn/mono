import { useRef, useCallback, useState } from 'react'

interface SwipeHandlers {
  onSwipeLeft?: () => void
  onSwipeRight?: () => void
  onSwipeUp?: () => void
  onSwipeDown?: () => void
}

interface TouchState {
  startX: number
  startY: number
  startTime: number
}

export function useSwipe(handlers: SwipeHandlers, threshold = 50) {
  const touchRef = useRef<TouchState | null>(null)

  const onTouchStart = useCallback((e: React.TouchEvent) => {
    const touch = e.touches[0]
    touchRef.current = {
      startX: touch.clientX,
      startY: touch.clientY,
      startTime: Date.now(),
    }
  }, [])

  const onTouchEnd = useCallback((e: React.TouchEvent) => {
    if (!touchRef.current) return
    const touch = e.changedTouches[0]
    const dx = touch.clientX - touchRef.current.startX
    const dy = touch.clientY - touchRef.current.startY
    const dt = Date.now() - touchRef.current.startTime

    if (dt > 500) return

    const absDx = Math.abs(dx)
    const absDy = Math.abs(dy)

    if (absDx > absDy && absDx > threshold) {
      if (dx > 0) handlers.onSwipeRight?.()
      else handlers.onSwipeLeft?.()
    } else if (absDy > absDx && absDy > threshold) {
      if (dy > 0) handlers.onSwipeDown?.()
      else handlers.onSwipeUp?.()
    }

    touchRef.current = null
  }, [handlers, threshold])

  return { onTouchStart, onTouchEnd }
}

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  title?: string
  children: React.ReactNode
}

export function BottomSheet({ open, onClose, title, children }: BottomSheetProps) {
  const [dragging, setDragging] = useState(false)
  const [translateY, setTranslateY] = useState(0)
  const startYRef = useRef(0)

  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    startYRef.current = e.touches[0].clientY
    setDragging(true)
  }, [])

  const handleTouchMove = useCallback((e: React.TouchEvent) => {
    if (!dragging) return
    const dy = e.touches[0].clientY - startYRef.current
    if (dy > 0) setTranslateY(dy)
  }, [dragging])

  const handleTouchEnd = useCallback(() => {
    setDragging(false)
    if (translateY > 100) {
      onClose()
    }
    setTranslateY(0)
  }, [translateY, onClose])

  if (!open) return null

  return (
    <>
      <div
        className="fixed inset-0 bg-black/50 z-40 animate-fade-in"
        onClick={onClose}
        aria-hidden="true"
      />
      <div
        className="fixed bottom-0 left-0 right-0 z-50 bg-surface rounded-t-2xl border-t border-border max-h-[80vh] overflow-hidden animate-slide-up"
        style={{ transform: `translateY(${translateY}px)`, transition: dragging ? 'none' : 'transform 0.2s ease-out' }}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div
          className="flex justify-center py-3 cursor-grab active:cursor-grabbing"
          onTouchStart={handleTouchStart}
          onTouchMove={handleTouchMove}
          onTouchEnd={handleTouchEnd}
        >
          <div className="w-10 h-1 rounded-full bg-border-2" aria-hidden="true" />
        </div>
        {title && (
          <div className="px-5 pb-3 border-b border-border">
            <h2 className="text-sm font-medium">{title}</h2>
          </div>
        )}
        <div className="overflow-auto p-5 max-h-[calc(80vh-4rem)]">
          {children}
        </div>
      </div>
    </>
  )
}
