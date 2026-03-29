import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

interface EmptyStateProps {
  icon?: React.ReactNode
  title: string
  description?: string
  action?: { label: string; onClick: () => void }
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <Card className="border-dashed">
      <CardContent className="py-12 flex flex-col items-center text-center">
        {icon && <div className="text-muted-foreground mb-3">{icon}</div>}
        <p className="font-medium">{title}</p>
        {description && <p className="text-sm text-muted-foreground mt-1 max-w-sm">{description}</p>}
        {action && (
          <Button onClick={action.onClick} className="mt-4" size="sm">
            {action.label}
          </Button>
        )}
      </CardContent>
    </Card>
  )
}
