import type { FC } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "./ui/button";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
} from "./ui/dialog";
import { useTranslation, Trans } from "react-i18next";

interface RelayConfirmationDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onConfirm: () => void;
    entityName: string;
    roomName: string;
}

export const RelayConfirmationDialog: FC<RelayConfirmationDialogProps> = ({
    open,
    onOpenChange,
    onConfirm,
    entityName,
    roomName,
}) => {
    const { t } = useTranslation();

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <div className="flex items-center gap-2 justify-center sm:justify-start">
                        <div className="p-2 bg-destructive/10 rounded-full">
                            <AlertTriangle className="w-5 h-5 text-destructive" />
                        </div>
                        <DialogTitle>{t("Confirmation")}</DialogTitle>
                    </div>
                    <DialogDescription className="pt-2">
                        <Trans
                            i18nKey="Are you sure you want to turn off {{entity}} in {{room}}?"
                            values={{ entity: entityName, room: roomName }}
                            components={{ bold: <strong /> }}
                        />
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        {t("Cancel")}
                    </Button>
                    <Button
                        variant="destructive"
                        onClick={() => {
                            onConfirm();
                            onOpenChange(false);
                        }}
                    >
                        {t("Confirm")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
};
