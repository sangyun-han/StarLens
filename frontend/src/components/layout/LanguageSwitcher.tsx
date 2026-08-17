import { Check, Languages } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { SUPPORTED_LANGUAGES } from '@/i18n'
import { cn } from '@/lib/utils'

/**
 * Language picker listing every auto-discovered locale by its native name.
 * The choice is persisted by the i18next language detector (localStorage).
 */
export function LanguageSwitcher() {
  const { t, i18n } = useTranslation()
  const active = i18n.resolvedLanguage ?? i18n.language

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t('common.changeLanguage')}
          className="text-muted-foreground"
        >
          <Languages className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {SUPPORTED_LANGUAGES.map((language) => (
          <DropdownMenuItem
            key={language.code}
            onClick={() => void i18n.changeLanguage(language.code)}
          >
            <Check
              className={cn(
                'size-3.5',
                language.code !== active && 'invisible',
              )}
            />
            {language.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
