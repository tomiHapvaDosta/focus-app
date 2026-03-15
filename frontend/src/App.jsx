import { useState } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import RequireAuth from './components/RequireAuth'
import TabBar from './components/TabBar'
import AddTaskSheet from './components/AddTaskSheet'
import LoginPage from './pages/LoginPage'
import SignupPage from './pages/SignupPage'
import TodayPage from './pages/TodayPage'
import SchedulePage from './pages/SchedulePage'
import RoutinePage from './pages/RoutinePage'
import PatternsPage from './pages/PatternsPage'

function AppShell() {
  const [addOpen, setAddOpen] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

  function handleAdded() {
    setRefreshKey(k => k + 1)
  }

  return (
    <div className="app-shell">
      <main className="app-main">
        <Routes>
          <Route path="/today" element={<TodayPage key={refreshKey} />} />
          <Route path="/schedule" element={<SchedulePage />} />
          <Route path="/routine" element={<RoutinePage />} />
          <Route path="/patterns" element={<PatternsPage />} />
          <Route path="*" element={<Navigate to="/today" replace />} />
        </Routes>
      </main>

      <TabBar onAddClick={() => setAddOpen(true)} />

      <AddTaskSheet
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onAdded={handleAdded}
      />
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