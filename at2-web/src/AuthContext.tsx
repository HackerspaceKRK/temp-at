import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { API_URL } from "./config";

interface User {
  username: string;
  membershipExpirationTimestamp?: number | null;
}

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  login: () => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const checkAuth = async () => {
    const apiUrl = API_URL.replace(/\/$/, "");
    let slowRequestCompleted = false;

    // Perform fast request (cache-only)
    fetch(`${apiUrl}/api/v1/auth/me?fast=true`)
      .then(async (res) => {
        if (slowRequestCompleted) return;
        if (res.ok) {
          const data = await res.json();
          if (!slowRequestCompleted) {
            setUser(data);
          }
        }
      })
      .catch(() => {
        // Ignore errors from fast endpoint
      });

    // Perform normal request (actual auth verification)
    try {
      const response = await fetch(`${apiUrl}/api/v1/auth/me`);
      slowRequestCompleted = true;
      if (response.ok) {
        const data = await response.json();
        setUser(data);
      } else {
        setUser(null);
      }
    } catch (error) {
      console.error("Auth check failed", error);
      slowRequestCompleted = true;
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    checkAuth();
  }, []);

  const login = () => {
    window.location.href = `${API_URL.replace(/\/$/, "")}/api/v1/auth/login`;
  };

  const logout = async () => {
    try {
      await fetch(`${API_URL.replace(/\/$/, "")}/api/v1/auth/logout`, {
        method: "POST",
      });
      setUser(null);
    } catch (error) {
      console.error("Logout failed", error);
    }
  };

  return (
    <AuthContext.Provider value={{ user, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
