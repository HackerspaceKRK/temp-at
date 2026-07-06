import { lazy, Suspense, type FC, type PropsWithChildren } from "react";
import "./app.css";
import Footer from "./components/Footer";
import { AuthProvider } from "./AuthContext";
import { AppConfigProvider } from "./AppConfigContext";
import { ThemeProvider } from "./theme";
import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router-dom";
import { AppNavbar } from "./components/AppNavbar";
import { TabletNavbar } from "./components/tablet/TabletNavbar";
import { LiveStateProvider } from "./useLiveRoomStates";
import { TabletAuthGate } from "./components/tablet/TabletAuthGate";
import { TabletSessionProvider } from "./components/tablet/TabletSessionContext";
import { InactivityRedirect } from "./components/tablet/InactivityRedirect";
import { ScreenSleepManager } from "./components/tablet/ScreenSleepManager";

const OverviewTabletPage = lazy(() =>
  import("./pages/OverviewTabletPage").then((m) => ({ default: m.OverviewTabletPage })),
);
const TabletDebugPage = lazy(() =>
  import("./pages/TabletDebugPage").then((m) => ({ default: m.TabletDebugPage })),
);
const TabletRoomPage = lazy(() =>
  import("./pages/TabletRoomPage").then((m) => ({ default: m.TabletRoomPage })),
);
const TabletPhonePage = lazy(() =>
  import("./pages/TabletPhonePage").then((m) => ({ default: m.TabletPhonePage })),
);
const RoomStatesPage = lazy(() =>
  import("./pages/RoomStatesPage").then((m) => ({ default: m.RoomStatesPage })),
);
const DhcpPage = lazy(() =>
  import("./pages/DhcpPage").then((m) => ({ default: m.DhcpPage })),
);
const PrintersPage = lazy(() =>
  import("./pages/PrintersPage").then((m) => ({ default: m.PrintersPage })),
);
const PrinterDetailPage = lazy(() =>
  import("./pages/PrinterDetailPage").then((m) => ({ default: m.PrinterDetailPage })),
);

const PageFallback: FC = () => (
  <div className="flex items-center justify-center py-24">
    <div className="h-8 w-8 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" />
  </div>
);

export function App() {
  return (
    <AppProviders>
      <BrowserRouter>
        <Suspense fallback={<PageFallback />}>
        <Routes>
          <Route element={<DefaultLayout />}>
            <Route path="/" element={<RoomStatesPage />} />
            <Route path="/dhcp" element={<DhcpPage />} />
            <Route path="/printers" element={<PrintersPage />} />
            {/* Splat route: printer ids may contain slashes */}
            <Route path="/printers/*" element={<PrinterDetailPage />} />
          </Route>
          <Route element={<TabletLayout />}>
            <Route path="/tablet/overview" element={<OverviewTabletPage />} />
            <Route path="/tablet/debug" element={<TabletDebugPage />} />
            <Route path="/tablet/phone" element={<TabletPhonePage />} />
            <Route path="/tablet/room/:id" element={<TabletRoomPage />} />
          </Route>
          {/* Redirect legacy / bare tablet URLs to the overview page */}
          <Route
            path="/tablet"
            element={<Navigate to="/tablet/overview" replace />}
          />
          <Route
            path="/tablet/entrance"
            element={<Navigate to="/tablet/overview" replace />}
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
        </Suspense>
      </BrowserRouter>
    </AppProviders>
  );
}

const AppProviders: FC<PropsWithChildren> = ({ children }) => {
  return (
    <ThemeProvider>
      <AuthProvider>
        <AppConfigProvider>
          <LiveStateProvider>{children}</LiveStateProvider>
        </AppConfigProvider>
      </AuthProvider>
    </ThemeProvider>
  );
};

const DefaultLayout: FC = () => (
  <div className="min-h-screen flex flex-col bg-background text-foreground">
    <div className="w-full flex-grow">
      <AppNavbar />
      <Outlet />
    </div>
    <Footer />
  </div>
);

const TabletLayout: FC = () => (
  <TabletAuthGate>
    <TabletSessionProvider>
      <div className="flex h-screen flex-col overflow-hidden bg-background text-foreground">
        <TabletNavbar />
        <Outlet />
        <InactivityRedirect />
        <ScreenSleepManager />
      </div>
    </TabletSessionProvider>
  </TabletAuthGate>
);
