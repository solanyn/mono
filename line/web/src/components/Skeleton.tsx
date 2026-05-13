import clsx from 'clsx'

interface SkeletonProps {
  className?: string
}

export function Skeleton({ className }: SkeletonProps) {
  return (
    <div
      className={clsx('animate-pulse rounded bg-border/50', className)}
      aria-hidden="true"
    />
  )
}

export function SessionCardSkeleton() {
  return (
    <div className="flex justify-between items-center p-4 bg-surface-2 rounded-lg border border-border animate-pulse">
      <div className="space-y-2">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-3 w-48" />
      </div>
      <div className="space-y-2 flex flex-col items-end">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-3 w-16" />
      </div>
    </div>
  )
}

export function LapListSkeleton() {
  return (
    <div className="space-y-1 p-3" aria-label="Loading laps">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-8 w-full" />
      ))}
    </div>
  )
}

export function ChartSkeleton() {
  return (
    <div className="p-4 space-y-3 animate-pulse" aria-label="Loading chart">
      <Skeleton className="h-3 w-24" />
      <Skeleton className="h-28 w-full rounded-md" />
      <Skeleton className="h-3 w-32 mt-4" />
      <Skeleton className="h-28 w-full rounded-md" />
    </div>
  )
}

export function MetricCardSkeleton() {
  return (
    <div className="p-3 bg-surface-2 rounded-lg border border-border animate-pulse">
      <Skeleton className="h-3 w-16 mb-2" />
      <Skeleton className="h-6 w-24" />
    </div>
  )
}
