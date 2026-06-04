import type { FC, PropsWithChildren } from "react";
import "./app.css";
import Footer from "./components/Footer";
import { AuthProvider } from "./AuthContext";
import { AppConfigProvider } from "./AppConfigContext";
import { ThemeProvider } from "./theme";
import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router-dom";
import { AppNavbar } from "./components/AppNavbar";
import { TabletNavbar } from "./components/tablet/TabletNavbar";
import { OverviewTabletPage } from "./pages/OverviewTabletPage";
import { TabletDebugPage } from "./pages/TabletDebugPage";
import { TabletRoomPage } from "./pages/TabletRoomPage";
import { RoomStatesPage } from "./pages/RoomStatesPage";
import { LiveStateProvider } from "./useLiveRoomStates";

export function App() {
  return (
    <AppProviders>
      <BrowserRouter>
        <Routes>
          <Route element={<DefaultLayout />}>
            <Route path="/" element={<RoomStatesPage />} />
          </Route>
          <Route element={<TabletLayout />}>
            <Route path="/tablet/overview" element={<OverviewTabletPage />} />
            <Route path="/tablet/debug" element={<TabletDebugPage />} />
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
  <div className="flex h-screen flex-col overflow-hidden bg-background text-foreground">
    <TabletNavbar />
    <Outlet />
  </div>
);
