import { Languages } from "lucide-react"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectIconTrigger,
} from "@/components/ui/select"
import { useTranslation } from "react-i18next"

export function LanguageToggle() {
    const { i18n, t } = useTranslation()

    return (
        <Select value={i18n.language} onValueChange={(val) => i18n.changeLanguage(val)}>
            <SelectIconTrigger>
                <Languages className="h-[1.2rem] w-[1.2rem]" />
                <span className="sr-only">{t("Toggle language")}</span>
            </SelectIconTrigger>
            <SelectContent align="end">
                <SelectItem value="en">English</SelectItem>
                <SelectItem value="pl">Polski</SelectItem>
            </SelectContent>
        </Select>
    )
}
