import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { ConfigProvider, theme as antdTheme } from 'antd'

import { runtimeConfig } from './config'
import { getThemeMode, onThemeChange, type ThemeMode } from './theme'
import AuthProvider from './auth/AuthContext'
import { RequireAuth } from './components/RequireAuth'
import AppLayout from './components/AppLayout'
import LoginPage from './pages/LoginPage'
import OAuthCallbackPage from './pages/OAuthCallbackPage'
import DashboardPage from './pages/DashboardPage'
import AlertsPage from './pages/AlertsPage'
import DeliveriesPage from './pages/DeliveriesPage'
import SourcesPage from './pages/SourcesPage'
import ChannelsPage from './pages/ChannelsPage'
import TemplatesPage from './pages/TemplatesPage'
import RulesPage from './pages/RulesPage'
import SettingsPage from './pages/SettingsPage'
import UsersPage from './pages/UsersPage'

export default function App() {
  const [mode, setMode] = useState<ThemeMode>(getThemeMode)
  useEffect(() => onThemeChange(() => setMode(getThemeMode())), [])

  return (
    <ConfigProvider
      theme={{ algorithm: mode === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm }}
    >
      <BrowserRouter basename={runtimeConfig.base}>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/oauth/callback" element={<OAuthCallbackPage />} />
          <Route
            element={
              <RequireAuth>
                <AppLayout />
              </RequireAuth>
            }
          >
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/alerts" element={<AlertsPage />} />
            <Route
              path="/deliveries"
              element={
                <RequireAuth roles={['editor', 'admin']}>
                  <DeliveriesPage />
                </RequireAuth>
              }
            />
            <Route path="/sources" element={<SourcesPage />} />
            <Route path="/channels" element={<ChannelsPage />} />
            <Route path="/templates" element={<TemplatesPage />} />
            <Route path="/rules" element={<RulesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route
              path="/users"
              element={
                <RequireAuth roles={['admin']}>
                  <UsersPage />
                </RequireAuth>
              }
            />
          </Route>
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </AuthProvider>
      </BrowserRouter>
    </ConfigProvider>
  )
}
