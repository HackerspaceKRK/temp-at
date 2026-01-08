import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { API_URL } from "./config";

export interface BrandingConfig {
    page_title?: string;
    logo_url?: string;
    logo_dark_url?: string;
    logo_alt?: string;
    logo_link_url?: string;
    footer_name?: string;
    footer_name_link_url?: string;
}

export interface VersionInfo {
    git_repo_url: string;
    git_commit_hash: string;
    git_commit_date: string;
}

export interface AppConfig {
    branding: BrandingConfig;
    version: VersionInfo;
}

interface AppConfigContextType {
    config: AppConfig | null;
    isLoading: boolean;
}

const AppConfigContext = createContext<AppConfigContextType | null>(null);

export function AppConfigProvider({ children }: { children: ReactNode }) {
    const [config, setConfig] = useState<AppConfig | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        fetch(`${API_URL.replace(/\/$/, "")}/api/v1/app-config`)
            .then((res) => {
                if (!res.ok) throw new Error("Failed to fetch app config");
                return res.json();
            })
            .then((data) => {
                setConfig(data);
                // Dynamically update document title if present
                if (data.branding?.page_title) {
                    document.title = data.branding.page_title;
                }
            })
            .catch((err) => {
                console.error("Error loading app config:", err);
            })
            .finally(() => {
                setIsLoading(false);
            });
    }, []);

    return (
        <AppConfigContext.Provider value={{ config, isLoading }}>
            {children}
        </AppConfigContext.Provider>
    );
}

export function useAppConfig() {
    const context = useContext(AppConfigContext);
    if (!context) {
        throw new Error("useAppConfig must be used within an AppConfigProvider");
    }
    return context;
}
