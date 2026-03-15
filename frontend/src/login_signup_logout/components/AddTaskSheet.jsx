import { useState, useEffect, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import * as api from '../api/client'

const AREA_CHIPS = ['Work', 'Personal', 'Health', 'Learning', 'Admin']
const DURATION_CHIPS = [
    { label: '5m', value: 5 },
    { label: '15m', value: 15 },
    { label: '30m', value: 30 },
    { label: '1h', value: 60 },
    { label: '2h', value: 120 },
]
const DAYS = [
    { label: 'Su', value: 0 },
    { label: 'Mo', value: 1 },
    { label: 'Tu', value: 2 },
    { label: 'We', value: 3 },
    { label: 'Th', value: 4 },
    { label: 'Fr', value: 5 },
    { label: 'Sa', value: 6 },
]

const EMPTY = {
    title: '',
    area: '',
    customArea: '',
    duration: 30,
    customDur: '',
    useCustomDur: false,
    priority: 3,
    type: '',
    subtype: '',
    recurrence: [],
    start_time: '',
    notes: '',
}

export default function AddTaskSheet({ open, onClose, onAdded }) {
    const { handleUnauthorized } = useAuth()
    const [form, setForm] = useState(EMPTY)
    const [error, setError] = useState('')
    const [saving, setSaving] = useState(false)
    const titleRef = useRef(null)

    useEffect(() => {
        if (open) {
            setForm(EMPTY)
            setError('')
            setTimeout(() => titleRef.current?.focus(), 80)
        }
    }, [open])

    function set(key, val) {
        setForm(prev => ({ ...prev, [key]: val }))
    }

    function toggleDay(d) {
        setForm(prev => {
            const has = prev.recurrence.includes(d)
            return {
                ...prev,
                recurrence: has
                    ? prev.recurrence.filter(x => x !== d)
                    : [...prev.recurrence, d],
            }
        })
    }

    const effectiveDuration = form.useCustomDur
        ? (parseInt(form.customDur) || 0)
        : form.duration

    async function handleSubmit() {
        setError('')
        if (!form.title.trim()) { setError('Title is required'); return }
        const area = form.area === '__custom' ? form.customArea.trim() : form.area
        if (!area) { setError('Area is required'); return }
        if (!form.type) { setError('Select a type'); return }
        if (!form.subtype) { setError('Select a subtype'); return }
        if (form.subtype === 'time-bound' && !form.start_time) {
            setError('Start time is required for time-bound tasks'); return
        }

        const payload = {
            title: form.title.trim(),
            area,
            duration: effectiveDuration,
            priority: form.priority,
            type: form.type,
            subtype: form.subtype,
            recurrence: form.type === 'recurring' ? form.recurrence : [],
            notes: form.notes.trim() || null,
            start_time: form.subtype === 'time-bound' ? form.start_time || null : null,
        }

        setSaving(true)
        try {
            const created = await api.createTask(payload)
            onAdded?.(created)
            onClose()
        } catch (err) {
            if (err.status === 401) { handleUnauthorized(); return }
            setError(err.message || 'Failed to create task')
        } finally {
            setSaving(false)
        }
    }

    if (!open) return null

    return (
        <div className="sheet-overlay" onClick={onClose}>
            <div className="sheet add-sheet" onClick={e => e.stopPropagation()}>
                <div className="sheet-handle" />

                <div className="sheet-scroll">
                    <div className="sheet-title">New task</div>

                    {/* Title */}
                    <div className="field">
                        <input
                            ref={titleRef}
                            className="field-input field-input-lg"
                            placeholder="What needs to be done?"
                            value={form.title}
                            onChange={e => set('title', e.target.value)}
                            onKeyDown={e => e.key === 'Enter' && handleSubmit()}
                        />
                    </div>

                    {/* Area */}
                    <div className="field">
                        <div className="field-label">Area</div>
                        <div className="chip-row">
                            {AREA_CHIPS.map(a => (
                                <button
                                    key={a}
                                    className={`chip${form.area === a ? ' chip-active' : ''}`}
                                    onClick={() => set('area', a)}
                                >{a}</button>
                            ))}
                            <button
                                className={`chip${form.area === '__custom' ? ' chip-active' : ''}`}
                                onClick={() => set('area', '__custom')}
                            >Other</button>
                        </div>
                        {form.area === '__custom' && (
                            <input
                                className="field-input"
                                style={{ marginTop: '8px' }}
                                placeholder="Custom area…"
                                value={form.customArea}
                                onChange={e => set('customArea', e.target.value)}
                                autoFocus
                            />
                        )}
                    </div>

                    {/* Duration */}
                    <div className="field">
                        <div className="field-label">Duration</div>
                        <div className="chip-row">
                            {DURATION_CHIPS.map(d => (
                                <button
                                    key={d.value}
                                    className={`chip${!form.useCustomDur && form.duration === d.value ? ' chip-active' : ''}`}
                                    onClick={() => { set('duration', d.value); set('useCustomDur', false) }}
                                >{d.label}</button>
                            ))}
                            <button
                                className={`chip${form.useCustomDur ? ' chip-active' : ''}`}
                                onClick={() => set('useCustomDur', true)}
                            >Custom</button>
                        </div>
                        {form.useCustomDur && (
                            <div className="field-row">
                                <input
                                    className="field-input field-input-sm"
                                    type="number"
                                    placeholder="minutes"
                                    value={form.customDur}
                                    onChange={e => set('customDur', e.target.value)}
                                    autoFocus
                                />
                                <span className="field-unit">min</span>
                            </div>
                        )}
                    </div>

                    {/* Priority */}
                    <div className="field">
                        <div className="field-label">Priority</div>
                        <div className="priority-row">
                            {[1, 2, 3, 4, 5].map(p => (
                                <button
                                    key={p}
                                    className={`priority-btn${form.priority === p ? ' priority-active' : ''}`}
                                    onClick={() => set('priority', p)}
                                >
                                    <span className="priority-num">{p}</span>
                                    <span className="priority-hint">
                                        {p === 1 ? 'urgent' : p === 2 ? 'high' : p === 3 ? 'medium' : p === 4 ? 'low' : 'someday'}
                                    </span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Type */}
                    <div className="field">
                        <div className="field-label">Type</div>
                        <div className="toggle-row">
                            <button
                                className={`toggle-btn${form.type === 'recurring' ? ' toggle-active' : ''}`}
                                onClick={() => { set('type', 'recurring'); set('subtype', '') }}
                            >Recurring</button>
                            <button
                                className={`toggle-btn${form.type === 'one-time' ? ' toggle-active' : ''}`}
                                onClick={() => { set('type', 'one-time'); set('subtype', '') }}
                            >One-time</button>
                        </div>
                    </div>

                    {/* Subtype */}
                    {form.type && (
                        <div className="field">
                            <div className="field-label">Schedule</div>
                            <div className="toggle-row">
                                <button
                                    className={`toggle-btn${form.subtype === 'time-bound' ? ' toggle-active' : ''}`}
                                    onClick={() => set('subtype', 'time-bound')}
                                >Time-bound</button>
                                <button
                                    className={`toggle-btn${form.subtype === 'flexible' ? ' toggle-active' : ''}`}
                                    onClick={() => set('subtype', 'flexible')}
                                >Flexible</button>
                            </div>
                        </div>
                    )}

                    {/* Recurrence days */}
                    {form.type === 'recurring' && (
                        <div className="field">
                            <div className="field-label">Repeats on</div>
                            <div className="day-row">
                                {DAYS.map(d => (
                                    <button
                                        key={d.value}
                                        className={`day-btn${form.recurrence.includes(d.value) ? ' day-active' : ''}`}
                                        onClick={() => toggleDay(d.value)}
                                    >{d.label}</button>
                                ))}
                            </div>
                        </div>
                    )}

                    {/* Start time */}
                    {form.subtype === 'time-bound' && (
                        <div className="field">
                            <div className="field-label">Start time</div>
                            <input
                                className="field-input field-input-time"
                                type="time"
                                value={form.start_time}
                                onChange={e => set('start_time', e.target.value)}
                            />
                        </div>
                    )}

                    {/* Notes */}
                    <div className="field">
                        <div className="field-label">
                            Notes <span className="field-optional">(optional)</span>
                        </div>
                        <textarea
                            className="field-textarea"
                            placeholder="Any context…"
                            rows={3}
                            value={form.notes}
                            onChange={e => set('notes', e.target.value)}
                        />
                    </div>

                    {/* Error */}
                    {error && <div className="sheet-error">{error}</div>}

                    {/* Submit */}
                    <button className="sheet-submit" onClick={handleSubmit} disabled={saving}>
                        {saving ? 'Adding…' : 'Add task'}
                    </button>
                </div>
            </div>
        </div>
    )
}