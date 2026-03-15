import { createContext, useContext, useState, useCallback, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
    const [token, setToken] = useState(() => localStorage.getItem('focus_token'))
    const navigate = useNavigate()

    const saveToken = useCallback((t) => {
        localStorage.setItem('focus_token', t)
        setToken(t)
    }, [])

    const clearToken = useCallback(() => {
        localStorage.removeItem('focus_token')
        setToken(null)
    }, [])

    const handleSignup = useCallback(async (email, password) => {
        const data = await api.signup(email, password)
        saveToken(data.token)
        navigate('/today', { replace: true })
    }, [saveToken, navigate])

    const handleLogin = useCallback(async (email, password) => {
        const data = await api.login(email, password)
        saveToken(data.token)
        navigate('/today', { replace: true })
    }, [saveToken, navigate])

    const handleLogout = useCallback(async () => {
        try { await api.logout() } catch (_) { }
        clearToken()
        navigate('/login', { replace: true })
    }, [clearToken, navigate])

    // If a protected API call returns 401 (token revoked/expired), bounce to login
    const handleUnauthorized = useCallback(() => {
        clearToken()
        navigate('/login', { replace: true })
    }, [clearToken, navigate])

    const value = {
        token,
        isAuthenticated: !!token,
        signup: handleSignup,
        login: handleLogin,
        logout: handleLogout,
        handleUnauthorized,
    }

    return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
    const ctx = useContext(AuthContext)
    if (!ctx) throw new Error('useAuth must be used within AuthProvider')
    return ctx
}