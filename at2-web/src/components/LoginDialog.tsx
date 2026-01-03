import type { FC } from "react";
import { Lock } from "lucide-react";
import { Button } from "./ui/button";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
} from "./ui/dialog";
import { useTranslation } from "react-i18next";

interface LoginDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onLogin: () => void;
}

export const LoginDialog: FC<LoginDialogProps> = ({
    open,
    onOpenChange,
    onLogin,
}) => {
    const { t } = useTranslation();

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <div className="flex items-center gap-2 justify-center sm:justify-start">
                        <div className="p-2 bg-primary/10 rounded-full">
                            <Lock className="w-5 h-5 text-primary" />
                        </div>
                        <DialogTitle>{t("Login Required")}</DialogTitle>
                    </div>
                    <DialogDescription className="pt-2">
                        {t("You must be logged in to control devices.")}
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        {t("Cancel")}
                    </Button>
                    <Button onClick={onLogin}>{t("Log In")}</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
};
