import { Moon, Sun } from "lucide-react"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectIconTrigger,
} from "@/components/ui/select"
import { useTheme } from "@/theme"
import { useTranslation } from "react-i18next"

export function ModeToggle() {
  const { theme, setTheme } = useTheme()
  const { t } = useTranslation()

  return (
    <Select value={theme} onValueChange={setTheme}>
      <SelectIconTrigger>
        <Sun className="h-[1.2rem] w-[1.2rem] scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90" />
        <Moon className="absolute h-[1.2rem] w-[1.2rem] scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0" />
        <span className="sr-only">{t("Toggle theme")}</span>
      </SelectIconTrigger>
      <SelectContent align="end">
        <SelectItem value="light">{t("Light")}</SelectItem>
        <SelectItem value="dark">{t("Dark")}</SelectItem>
        <SelectItem value="system">{t("System")}</SelectItem>
      </SelectContent>
    </Select>
  )
}
