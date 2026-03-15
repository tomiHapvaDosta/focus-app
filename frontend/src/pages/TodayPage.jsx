import { useEffect, useState, useRef, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import * as api from '../api/client'

const DAY_NAMES = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const PRIORITY_COLOR = { 1: '#ff5f5f', 2: '#f0a050', 3: '#6b7cff', 4: '#4caf7d', 5: '#55555f' }

function fmtDuration(min) {
    if (!min) return '—'
    if (min < 60) return `${min}m`
    const h = Math.floor(min / 60), m = min % 60
    return m ? `${h}h ${m}m` : `${h}h`
}

function fmtTime(hhmm) {
    if (!hhmm) return ''
    const [h, m] = hhmm.split(':').map(Number)
    const ampm = h >= 12 ? 'pm' : 'am'
    const hour = h % 12 || 12
    return m ? `${hour}:${String(m).padStart(2, '0')}${ampm}` : `${hour}${ampm}`
}

function toMinutes(hhmm) {
    if (!hhmm) return 0
    const [h, m] = hhmm.split(':').map(Number)
    return h * 60 + m
}

const isTouchDevice = () =>
    typeof window !== 'undefined' &&
    ('ontouchstart' in window || navigator.maxTouchPoints > 0)

// ── Swipeable / draggable task card ──────────────────────────────────────────
function TaskCard({ task, onComplete, isConflict, expanded, onToggleExpand }) {
    const startX = useRef(null)
    const dragging = useRef(false)
    const [dx, setDx] = useState(0)
    const [completing, setCompleting] = useState(false)

    const THRESHOLD = 72

    // ── touch ──────────────────────────────────────────────────────────────
    function onTouchStart(e) { startX.current = e.touches[0].clientX }
    function onTouchMove(e) {
        if (startX.current === null) return
        const d = e.touches[0].clientX - startX.current
        if (d > 0) setDx(Math.min(d, 120))
    }
    async function onTouchEnd() {
        if (dx > THRESHOLD) { setCompleting(true); await onComplete(task.id) }
        setDx(0); startX.current = null
    }

    // ── mouse ──────────────────────────────────────────────────────────────
    function onMouseDown(e) {
        startX.current = e.clientX
        dragging.current = false

        function onMouseMove(ev) {
            const d = ev.clientX - startX.current
            if (d > 5) dragging.current = true
            if (d > 0) setDx(Math.min(d, 120))
        }
        async function onMouseUp() {
            window.removeEventListener('mousemove', onMouseMove)
            window.removeEventListener('mouseup', onMouseUp)
            if (dx > THRESHOLD) { setCompleting(true); await onComplete(task.id) }
            setDx(0); startX.current = null
        }
        window.addEventListener('mousemove', onMouseMove)
        window.addEventListener('mouseup', onMouseUp)
    }

    function handleClick() {
        // don't expand if the user just dragged
        if (dragging.current) { dragging.current = false; return }
        onToggleExpand(task.id)
    }

    const priorityColor = PRIORITY_COLOR[task.priority] || '#55555f'
    const swipeProgress = Math.min(dx / THRESHOLD, 1)

    return (
        <div className="task-swipe-container">
            <div className="swipe-bg">
                <svg
                    width="18" height="18" viewBox="0 0 24 24" fill="none"
                    stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                    style={{ opacity: swipeProgress }}
                >
                    <polyline points="20 6 9 17 4 12" />
                </svg>
            </div>

            <div
                className={`task-card${isConflict ? ' task-card-conflict' : ''}${completing ? ' task-completing' : ''}`}
                style={{
                    transform: `translateX(${dx}px)`,
                    transition: dx === 0 ? 'transform 0.22s ease' : 'none',
                    cursor: isTouchDevice() ? 'default' : 'grab',
                    userSelect: 'none',
                }}
                onTouchStart={onTouchStart}
                onTouchMove={onTouchMove}
                onTouchEnd={onTouchEnd}
                onMouseDown={onMouseDown}
                onClick={handleClick}
            >
                <div className="task-card-row">
                    <span className="task-priority-dot" style={{ background: priorityColor }} />
                    <div className="task-card-body">
                        <div className="task-card-title">{task.title}</div>
                        <div className="task-card-meta">
                            <span className="task-area-chip">{task.area}</span>
                            <span>{fmtDuration(task.duration)}</span>
                            {task.start_time && <span className="task-time">{fmtTime(task.start_time)}</span>}
                        </div>
                    </div>
                    <div className="task-card-right">
                        <span className="task-priority-badge" style={{ color: priorityColor }}>P{task.priority}</span>
                        {!isTouchDevice() && (
                            <span className="task-complete-hint" title="Drag right to complete">→ complete</span>
                        )}
                    </div>
                </div>

                {isConflict && (
                    <div className="task-conflict-banner">
                        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                            <line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" />
                        </svg>
                        Schedule conflict — overlaps with another task
                    </div>
                )}

                {expanded && (
                    <div className="task-notes">
                        {task.notes
                            ? task.notes
                            : <span className="task-notes-empty">No notes</span>
                        }
                    </div>
                )}
            </div>
        </div>
    )
}

// ── Gap strip ────────────────────────────────────────────────────────────────
function GapStrip({ gap }) {
    return (
        <div className={`gap-strip${!gap.best_fit ? ' gap-strip-empty' : ''}`}>
            <div className="gap-time-range">{fmtTime(gap.from)} – {fmtTime(gap.to)} · {gap.minutes_free}m free</div>
            {gap.best_fit
                ? <div className="gap-fit">↳ <strong>{gap.best_fit.title}</strong> fits · {fmtDuration(gap.best_fit.duration)}</div>
                : <div className="gap-fit gap-fit-none">No flexible task fits this window</div>
            }
        </div>
    )
}

// ── Hero card ─────────────────────────────────────────────────────────────────
function HeroCard({ task, onComplete }) {
    const [tapped, setTapped] = useState(false)
    const priorityColor = PRIORITY_COLOR[task.priority] || '#6b7cff'

    async function handleDone() {
        if (tapped) return
        setTapped(true)
        await onComplete(task.id)
    }

    return (
        <div className="hero-card" style={{ '--hero-accent': priorityColor }}>
            <div className="hero-eyebrow">do this next</div>
            <div className="hero-title">{task.title}</div>
            <div className="hero-meta">
                <span className="hero-area">{task.area}</span>
                {task.duration > 0 && <><span className="hero-sep">·</span><span>{fmtDuration(task.duration)}</span></>}
                {task.start_time && <><span className="hero-sep">·</span><span>{fmtTime(task.start_time)}</span></>}
            </div>
            <button className="hero-done-btn" onClick={handleDone} disabled={tapped}>
                {tapped ? '✓ Done' : 'Mark complete'}
            </button>
        </div>
    )
}

// ── Page ──────────────────────────────────────────────────────────────────────
export default function TodayPage() {
    const { handleUnauthorized } = useAuth()
    const [tasks, setTasks] = useState([])
    const [gaps, setGaps] = useState([])
    const [conflictIds, setConflictIds] = useState(new Set())
    const [expandedId, setExpandedId] = useState(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')

    const today = new Date()
    const dateLabel = `${DAY_NAMES[today.getDay()]}, ${today.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}`
    const touch = isTouchDevice()

    const load = useCallback(async () => {
        try {
            const [todayData, gapData, conflictData] = await Promise.all([
                api.getToday(),
                api.getGaps(),
                api.getScheduleConflicts(),
            ])
            setTasks(todayData || [])
            setGaps(gapData || [])
            const ids = new Set((conflictData || []).flatMap(c => [c.task_id_1, c.task_id_2]))
            setConflictIds(ids)
        } catch (err) {
            if (err.status === 401) handleUnauthorized()
            else setError('Could not load tasks')
        } finally {
            setLoading(false)
        }
    }, [handleUnauthorized])

    useEffect(() => { load() }, [load])

    async function handleComplete(id) {
        try {
            await api.completeTask(id)
            setTasks(prev => prev.filter(t => t.id !== id))
        } catch (err) {
            if (err.status === 401) handleUnauthorized()
        }
    }

    function toggleExpand(id) {
        setExpandedId(prev => prev === id ? null : id)
    }

    // hero = first task; exclude it from the sections below to avoid duplicate
    const hero = tasks[0] ?? null
    const heroId = hero?.id ?? null
    const rest = tasks.filter(t => t.id !== heroId)
    const timeBound = rest.filter(t => t.subtype === 'time-bound')
    const flexible = rest.filter(t => t.subtype === 'flexible')

    function buildTimeline() {
        const sorted = [...timeBound].sort((a, b) => toMinutes(a.start_time) - toMinutes(b.start_time))
        const items = []
        sorted.forEach(task => {
            items.push({ kind: 'task', data: task })
            if (task.start_time) {
                const taskEndMin = toMinutes(task.start_time) + (task.duration || 0)
                const matchGap = gaps.find(g => Math.abs(toMinutes(g.from) - taskEndMin) <= 5)
                if (matchGap) items.push({ kind: 'gap', data: matchGap })
            }
        })
        return items
    }

    if (loading) return (
        <div className="page-loading">
            <div className="loading-dots"><span /><span /><span /></div>
        </div>
    )

    return (
        <div className="today-page">
            <header className="today-header">
                <div className="today-date">{dateLabel}</div>
                <div className="today-task-count">{tasks.length} left</div>
            </header>

            {error && <div className="inline-error">{error}</div>}

            {tasks.length === 0 && !error && (
                <div className="today-empty">
                    <div className="today-empty-glyph">✦</div>
                    <div className="today-empty-title">All clear</div>
                    <div className="today-empty-sub">
                        {touch ? 'Tap + to add a task' : 'Click + to add a task'}
                    </div>
                </div>
            )}

            {hero && (
                <section className="today-section">
                    <HeroCard task={hero} onComplete={handleComplete} />
                </section>
            )}

            {timeBound.length > 0 && (
                <section className="today-section">
                    <div className="section-label">Scheduled</div>
                    {!touch && (
                        <div className="section-hint">Drag a card right to complete it</div>
                    )}
                    <div className="task-list">
                        {buildTimeline().map((item, i) =>
                            item.kind === 'gap'
                                ? <GapStrip key={`gap-${i}`} gap={item.data} />
                                : <TaskCard
                                    key={item.data.id}
                                    task={item.data}
                                    onComplete={handleComplete}
                                    isConflict={conflictIds.has(item.data.id)}
                                    expanded={expandedId === item.data.id}
                                    onToggleExpand={toggleExpand}
                                />
                        )}
                    </div>
                </section>
            )}

            {flexible.length > 0 && (
                <section className="today-section">
                    <div className="section-label">Flexible</div>
                    {!touch && flexible.length > 0 && timeBound.length === 0 && (
                        <div className="section-hint">Drag a card right to complete it</div>
                    )}
                    <div className="task-list">
                        {flexible.map(task => (
                            <TaskCard
                                key={task.id}
                                task={task}
                                onComplete={handleComplete}
                                isConflict={conflictIds.has(task.id)}
                                expanded={expandedId === task.id}
                                onToggleExpand={toggleExpand}
                            />
                        ))}
                    </div>
                </section>
            )}
        </div>
    )
}