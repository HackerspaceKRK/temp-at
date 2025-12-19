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
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <div className="flex items-center gap-2 justify-center sm:justify-start">
                        <div className="p-2 bg-destructive/10 rounded-full">
                            <AlertTriangle className="w-5 h-5 text-destructive" />
                        </div>
                        <DialogTitle>Potwierdzenie</DialogTitle>
                    </div>
                    <DialogDescription className="pt-2">
                        Czy na pewno chcesz wyłączyć <strong>{entityName}</strong> w{" "}
                        <strong>{roomName}</strong>? Upewnij się, że wyłączenia światła nie spowoduje niebezpieczeństwa.
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Anuluj
                    </Button>
                    <Button
                        variant="destructive"
                        onClick={() => {
                            onConfirm();
                            onOpenChange(false);
                        }}
                    >
                        Potwierdź
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
};
