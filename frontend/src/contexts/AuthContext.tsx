import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import api from '../services/api';

interface User {
    id: string;
    email: string;
    name?: string;
    created_at: string;
}

interface AuthContextType {
    user: User | null;
    token: string | null;
    isAuthenticated: boolean;
    isLoading: boolean;
    login: (email: string, password: string) => Promise<void>;
    register: (email: string, password: string, name?: string) => Promise<void>;
    logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [token, setToken] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    // Load token from localStorage on mount
    useEffect(() => {
        const savedToken = localStorage.getItem('auth_token');
        if (savedToken) {
            setToken(savedToken);
            api.defaults.headers.common['Authorization'] = `Bearer ${savedToken}`;
            // Fetch current user
            fetchUser(savedToken);
        } else {
            setIsLoading(false);
        }
    }, []);

    const fetchUser = async (authToken: string) => {
        try {
            api.defaults.headers.common['Authorization'] = `Bearer ${authToken}`;
            const response = await api.get('/auth/me');
            setUser(response.data);
        } catch {
            // Token is invalid, clear it
            localStorage.removeItem('auth_token');
            setToken(null);
            delete api.defaults.headers.common['Authorization'];
        } finally {
            setIsLoading(false);
        }
    };

    const login = async (email: string, password: string) => {
        const response = await api.post('/auth/login', { email, password });
        const { token: newToken, user: newUser } = response.data;

        setToken(newToken);
        setUser(newUser);
        localStorage.setItem('auth_token', newToken);
        api.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;
    };

    const register = async (email: string, password: string, name?: string) => {
        const response = await api.post('/auth/register', { email, password, name });
        const { token: newToken, user: newUser } = response.data;

        setToken(newToken);
        setUser(newUser);
        localStorage.setItem('auth_token', newToken);
        api.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;
    };

    const logout = () => {
        setToken(null);
        setUser(null);
        localStorage.removeItem('auth_token');
        delete api.defaults.headers.common['Authorization'];
    };

    return (
        <AuthContext.Provider
            value={{
                user,
                token,
                isAuthenticated: !!token,
                isLoading,
                login,
                register,
                logout,
            }}
        >
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
}
