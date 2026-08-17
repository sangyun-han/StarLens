import type { LucideIcon } from 'lucide-react'

import { Card, CardContent } from '@/components/ui/card'

interface PagePlaceholderProps {
  icon: LucideIcon
  title: string
  description: string
  /** What this page will contain once built. */
  planned: readonly string[]
}

/** Honest placeholder for routes that are wired but not implemented yet. */
export function PagePlaceholder({
  icon: Icon,
  title,
  description,
  planned,
}: PagePlaceholderProps) {
  return (
    <Card>
      <CardContent className="flex flex-col items-start gap-4 py-6">
        <span className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Icon className="size-5" />
        </span>
        <div>
          <h2 className="font-heading text-base font-semibold text-foreground">
            {title}
          </h2>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        <ul className="flex flex-col gap-1.5 text-sm text-muted-foreground">
          {planned.map((item) => (
            <li key={item} className="flex items-start gap-2">
              <span className="mt-1.75 size-1.5 shrink-0 rounded-full bg-border" />
              {item}
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}
