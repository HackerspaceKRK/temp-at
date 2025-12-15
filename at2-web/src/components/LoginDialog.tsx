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
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <div className="flex items-center gap-2 justify-center sm:justify-start">
                        <div className="p-2 bg-primary/10 rounded-full">
                            <Lock className="w-5 h-5 text-primary" />
                        </div>
                        <DialogTitle>Logowanie Wymagane</DialogTitle>
                    </div>
                    <DialogDescription className="pt-2">
                        Musisz być zalogowany, aby sterować urządzeniami.
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Anuluj
                    </Button>
                    <Button onClick={onLogin}>Zaloguj się</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
};
