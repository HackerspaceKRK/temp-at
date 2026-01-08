import type { FC } from "react";
import { Trans, useTranslation } from "react-i18next";
import { useAppConfig } from "../AppConfigContext";

const Footer: FC = () => {
    const { t } = useTranslation();
    const { config } = useAppConfig();
    const currentYear = new Date().getFullYear();
    const branding = config?.branding;
    const version = config?.version;

    if (!branding) return null;

    const shortHash = version?.git_commit_hash?.substring(0, 7) || "unknown";
    const commitDate = version?.git_commit_date ? new Date(version.git_commit_date).toLocaleString() : "";
    const commitUrl = version?.git_repo_url && version?.git_commit_hash ? `${version.git_repo_url}/commit/${version.git_commit_hash}` : undefined;

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
                {version && version.git_commit_hash !== "unknown" && (
                    <div className="text-xs opacity-50">
                        {t("Version")}:{" "}
                        <a
                            href={commitUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="hover:underline"
                        >
                            {shortHash} ({commitDate})
                        </a>
                    </div>
                )}
            </div>
        </footer>
    );
};

export default Footer;
