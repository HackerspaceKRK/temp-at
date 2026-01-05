import type { FC } from "react";
import { Trans, useTranslation } from "react-i18next";
import type { Branding } from "../schema";

interface FooterProps {
    branding?: Branding | null;
}

const Footer: FC<FooterProps> = ({ branding }) => {
    const { t } = useTranslation();
    const currentYear = new Date().getFullYear();

    if (!branding) return null;

    return (
        <footer className="mt-auto py-8 border-t border-border">
            <div className="container mx-auto px-4 flex flex-col items-center gap-2 text-sm text-muted-foreground">
                <div>
                    © {currentYear}{" "}
                    <a
                        href={branding.footer_name_link_url}
                        className="hover:underline text-foreground"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        {branding.footer_name}
                    </a>
                </div>
                <div className="flex flex-col sm:flex-row gap-2 sm:gap-4 items-center opacity-70">
                    <span>
                        <Trans i18nKey="footer.spaceapi">
                            We provide a <a
                                href="https://spaceapi.io/"
                                className="hover:underline text-foreground"
                                target="_blank"
                                rel="noopener noreferrer"
                            >SpaceAPI</a> endpoint: <a
                                href="/api/v1/spaceapi"
                                className="hover:underline text-foreground"
                                target="_blank"
                            >/api/v1/spaceapi</a>
                        </Trans>
                    </span>
                    <div className="flex gap-4">
                        <span>{t("Licensed under MIT")}</span>
                        <a
                            href="https://github.com/HackerspaceKRK/temp-at"
                            className="hover:underline text-foreground"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            {t("Source code")}
                        </a>
                    </div>
                </div>
            </div>
        </footer>
    );
};

export default Footer;
