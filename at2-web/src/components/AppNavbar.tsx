import { useState, type FC } from "react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";
import { DoorOpen, ExternalLink, LogIn, LogOut, Menu } from "lucide-react";
import { useAppConfig } from "../AppConfigContext";
import { useAuth } from "../AuthContext";
import { useLiveRoomStates } from "../useLiveRoomStates";
import { useLocale } from "../locale";
import { cn } from "../lib/utils";
import { useTheme } from "../theme";
import { LanguageToggle } from "./LanguageToggle";
import { ModeToggle } from "./ModeToggle";
import UserControls, { UserSummary } from "./UserControls";
import { Button } from "./ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SheetTrigger,
} from "./ui/sheet";

const navItemClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    "rounded-md px-3 py-2 text-sm font-medium transition-colors",
    isActive
      ? "bg-accent text-accent-foreground"
      : "text-muted-foreground hover:text-foreground",
  );

const Logo: FC<{ className?: string }> = ({ className }) => {
  const { t } = useTranslation();
  const { theme } = useTheme();
  const { config } = useAppConfig();
  const branding = config?.branding;

  const isDarkMode =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  const logoUrl =
    isDarkMode && branding?.logo_dark_url
      ? branding.logo_dark_url
      : branding?.logo_url;

  return (
    <a
      href={branding?.logo_link_url || "/"}
      className="flex shrink-0 items-center"
      target={branding?.logo_link_url?.startsWith("http") ? "_blank" : undefined}
      rel={
        branding?.logo_link_url?.startsWith("http")
          ? "noopener noreferrer"
          : undefined
      }
    >
      {logoUrl ? (
        <img
          src={logoUrl}
          alt={branding?.logo_alt || t("Logo")}
          className={cn("w-auto transition-transform hover:scale-105", className)}
        />
      ) : (
        <h1 className="text-2xl font-bold">{t("Headquarters")}</h1>
      )}
    </a>
  );
};

/** The page links, shared by the desktop bar and the mobile sidebar. */
const NavItems: FC<{ vertical?: boolean; onNavigate?: () => void }> = ({
  vertical,
  onNavigate,
}) => {
  const { t } = useTranslation();
  const { config } = useAppConfig();
  const { getName } = useLocale();
  const roomStates = useLiveRoomStates();
  const hasPrinters = roomStates.some((room) =>
    room.entities.some((entity) => entity.type === "printer"),
  );

  return (
    <nav
      className={cn(
        "flex gap-1",
        vertical
          ? "flex-col items-stretch"
          : "flex-wrap items-center justify-center",
      )}
    >
      <NavLink to="/" end className={navItemClass} onClick={onNavigate}>
        {t("Room Status")}
      </NavLink>
      {hasPrinters && (
        <NavLink to="/printers" className={navItemClass} onClick={onNavigate}>
          {t("Printers")}
        </NavLink>
      )}
      <NavLink to="/dhcp" className={navItemClass} onClick={onNavigate}>
        {t("DHCP")}
      </NavLink>
      {config?.nav_links?.map((link) => (
        <a
          key={link.url}
          href={link.url}
          target="_blank"
          rel="noopener noreferrer"
          onClick={onNavigate}
          className={cn(
            navItemClass({ isActive: false }),
            "inline-flex items-center gap-1",
          )}
        >
          {getName(link.localized_name, link.name) || link.name}
          <ExternalLink className="h-3.5 w-3.5" />
        </a>
      ))}
    </nav>
  );
};

/** Mobile top bar: logo + menu button, with the user's name and membership
 * expiration always visible; everything else lives in the sidebar. */
const MobileNavbar: FC = () => {
  const { t } = useTranslation();
  const { user, isLoading, login, logout } = useAuth();
  const { config } = useAppConfig();
  const [open, setOpen] = useState(false);
  const close = () => setOpen(false);

  return (
    <header className="flex items-center gap-3 px-4 py-3 md:hidden">
      <Logo className="max-h-10 max-w-[30vw] object-contain" />
      <div className="ml-auto min-w-0">
        {user ? (
          <UserSummary compact />
        ) : (
          !isLoading && (
            <Button onClick={login} size="sm">
              {t("Log In")}
            </Button>
          )
        )}
      </div>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetTrigger asChild>
          <Button variant="ghost" size="icon" className="-mr-2">
            <Menu className="size-5" />
            <span className="sr-only">{t("Open menu")}</span>
          </Button>
        </SheetTrigger>
        <SheetContent side="right">
          <SheetTitle className="sr-only">{t("Menu")}</SheetTitle>
          <SheetDescription className="sr-only">{t("Navigation")}</SheetDescription>
          <div className="pr-8">
            <Logo className="max-h-12" />
          </div>
          {user && <UserSummary className="rounded-md bg-muted/50 p-2" />}
          <NavItems vertical onNavigate={close} />
          <div className="h-px bg-border" />
          <div className="flex flex-col gap-1">
            {user ? (
              <>
                {config?.has_emergency_entry && (
                  <NavLink
                    to="/emergency-entry"
                    onClick={close}
                    className={(s) =>
                      cn(navItemClass(s), "inline-flex items-center gap-2")
                    }
                  >
                    <DoorOpen className="size-4" />
                    {t("Emergency entry")}
                  </NavLink>
                )}
                {config?.has_emergency_entry && (
                  <div className="my-1 h-px bg-border" />
                )}
                <button
                  type="button"
                  onClick={() => {
                    close();
                    logout();
                  }}
                  className={cn(
                    navItemClass({ isActive: false }),
                    "inline-flex items-center gap-2 text-left",
                  )}
                >
                  <LogOut className="size-4" />
                  {t("Log Out")}
                </button>
              </>
            ) : (
              !isLoading && (
                <Button onClick={login} size="sm" className="justify-start">
                  <LogIn />
                  {t("Log In")}
                </Button>
              )
            )}
          </div>
          <div className="mt-auto flex items-center gap-2">
            <LanguageToggle />
            <ModeToggle />
          </div>
        </SheetContent>
      </Sheet>
    </header>
  );
};

export const AppNavbar: FC = () => (
  <>
    <MobileNavbar />
    <header className="hidden px-4 py-4 md:flex flex-row items-center justify-between gap-4">
      <div className="flex flex-row items-center gap-6">
        <Logo className="max-h-[70px]" />
        <NavItems />
      </div>
      <div className="flex items-center gap-4 flex-wrap justify-center">
        <div className="flex items-center gap-2">
          <LanguageToggle />
          <ModeToggle />
        </div>
        <div className="w-px h-8 bg-border mx-2" />
        <UserControls />
      </div>
    </header>
  </>
);
