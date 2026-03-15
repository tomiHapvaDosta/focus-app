const BASE = 'http://localhost:8080'

function getToken() {
    return localStorage.getItem('focus_token')
}

async function request(method, path, body) {
    const headers = { 'Content-Type': 'application/json' }
    const token = getToken()
    if (token) headers['Authorization'] = `Bearer ${token}`

    const res = await fetch(`${BASE}${path}`, {
        method,
        headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
    })

    // 204 No Content
    if (res.status === 204) return null

    const text = await res.text()

    if (!res.ok) {
        // backend returns plain text errors via http.Error()
        throw { status: res.status, message: text.trim() }
    }

    try {
        return JSON.parse(text)
    } catch {
        return text
    }
}

// ── auth ─────────────────────────────────────────────────────────────────────
// POST /auth/signup  → { token: string }
export const signup = (email, password) =>
    request('POST', '/auth/signup', { email, password })

// POST /auth/login   → { token: string }
export const login = (email, password) =>
    request('POST', '/auth/login', { email, password })

// POST /auth/logout  → { message: string }
export const logout = () =>
    request('POST', '/auth/logout')

// ── tasks ────────────────────────────────────────────────────────────────────
// GET /tasks/today   → Task[]
export const getToday = () =>
    request('GET', '/tasks/today')

// GET /tasks/flexible → Task[]
export const getFlexible = () =>
    request('GET', '/tasks/flexible')

// GET /tasks/gaps    → Gap[]
// Gap: { from, to, minutes_free, best_fit: Task|null }
export const getGaps = () =>
    request('GET', '/tasks/gaps')

// POST /tasks        → Task
// body: { title, area, duration, priority, notes?, type, subtype, recurrence, start_time? }
export const createTask = (task) =>
    request('POST', '/tasks', task)

// PUT /tasks/:id     → Task
export const updateTask = (id, task) =>
    request('PUT', `/tasks/${id}`, task)

// DELETE /tasks/:id  → null (204)
export const deleteTask = (id) =>
    request('DELETE', `/tasks/${id}`)

// POST /tasks/:id/complete → { status: "completed" }
export const completeTask = (id) =>
    request('POST', `/tasks/${id}/complete`)

// ── schedule ──────────────────────────────────────────────────────────────────
// GET /schedule/:day (0=Sun … 6=Sat) → Task[]
export const getScheduleDay = (day) =>
    request('GET', `/schedule/${day}`)

// GET /schedule/conflicts → ConflictLog[]
// ConflictLog: { id, task_id_1, task_id_2, conflict_time, day_of_week, logged_at }
export const getScheduleConflicts = () =>
    request('GET', '/schedule/conflicts')

// ── errors ────────────────────────────────────────────────────────────────────
// GET /errors → ConflictLog[] ordered by logged_at DESC
export const getErrors = () =>
    request('GET', '/errors')

// ── patterns ─────────────────────────────────────────────────────────────────
// GET /patterns → PatternsByDay[]
// PatternsByDay: { day_of_week, day_name, patterns: PatternEntry[] }
// PatternEntry: { task_id, title, area, frequency_pct, day_of_week }
export const getPatterns = () =>
    request('GET', '/patterns')

// POST /patterns/:id/promote → Task
export const promotePattern = (id) =>
    request('POST', `/patterns/${id}/promote`)