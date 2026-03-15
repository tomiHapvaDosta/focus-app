import { useState } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import RequireAuth from './components/RequireAuth'
import TabBar from './components/TabBar'
import LoginPage from './pages/LoginPage'
import SignupPage from './pages/SignupPage'
import TodayPage from './pages/TodayPage'
import SchedulePage from './pages/SchedulePage'
import RoutinePage from './pages/RoutinePage'
import PatternsPage from './pages/PatternsPage'

function AppShell() {
  const [addOpen, setAddOpen] = useState(false)
  const { logout } = useAuth()

  return (
    <div className="app-shell">
      <main className="app-main">
        <Routes>
          <Route path="/today" element={<TodayPage />} />
          <Route path="/schedule" element={<SchedulePage />} />
          <Route path="/routine" element={<RoutinePage />} />
          <Route path="/patterns" element={<PatternsPage />} />
          <Route path="*" element={<Navigate to="/today" replace />} />
        </Routes>
      </main>

      <TabBar onAddClick={() => setAddOpen(true)} />

      {/* Add task sheet placeholder — replaced in Prompt 2 */}
      {addOpen && (
        <div className="sheet-overlay" onClick={() => setAddOpen(false)}>
          <div className="sheet" onClick={e => e.stopPropagation()}>
            <div className="sheet-handle" />
            <p style={{ padding: '2rem', color: 'var(--text-2)', textAlign: 'center' }}>
              Add task sheet — coming in Prompt 2
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/signup" element={<SignupPage />} />
      <Route
        path="/*"
        element={
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        }
      />
    </Routes>
  )
}
