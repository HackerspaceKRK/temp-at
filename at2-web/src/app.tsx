import type { FC, PropsWithChildren } from "react";
import "./app.css";
import Footer from "./components/Footer";
import { AuthProvider } from "./AuthContext";
import { AppConfigProvider } from "./AppConfigContext";
import { ThemeProvider } from "./theme";
import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router-dom";
import { AppNavbar } from "./components/AppNavbar";
import { TabletNavbar } from "./components/TabletNavbar";
import { EntranceTabletPage } from "./pages/EntranceTabletPage";
import { RoomStatesPage } from "./pages/RoomStatesPage";

export function App() {
  return (
    <AppProviders>
      <BrowserRouter>
        <Routes>
          <Route element={<DefaultLayout />}>
            <Route path="/" element={<RoomStatesPage />} />
          </Route>
          <Route element={<TabletLayout />}>
            <Route path="/tablet/entrance" element={<EntranceTabletPage />} />
          </Route>
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
        <AppConfigProvider>{children}</AppConfigProvider>
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
