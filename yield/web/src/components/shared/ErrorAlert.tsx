interface ErrorAlertProps {
  message: string
  className?: string
}

export function ErrorAlert({ message, className = '' }: ErrorAlertProps) {
  if (!message) return null
  return (
    <div className={`rounded-md border border-destructive bg-destructive/10 p-4 text-sm text-destructive ${className}`}>
      {message}
    </div>
  )
}
