import { NavLink, useLocation } from 'react-router-dom'

const tabs = [
    {
        to: '/today',
        label: 'Today',
        icon: (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
                <line x1="16" y1="2" x2="16" y2="6" />
                <line x1="8" y1="2" x2="8" y2="6" />
                <line x1="3" y1="10" x2="21" y2="10" />
            </svg>
        ),
    },
    {
        to: '/schedule',
        label: 'Schedule',
        icon: (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <polyline points="12 6 12 12 16 14" />
            </svg>
        ),
    },
]

const rightTabs = [
    {
        to: '/routine',
        label: 'Routine',
        icon: (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="17 1 21 5 17 9" />
                <path d="M3 11V9a4 4 0 0 1 4-4h14" />
                <polyline points="7 23 3 19 7 15" />
                <path d="M21 13v2a4 4 0 0 1-4 4H3" />
            </svg>
        ),
    },
    {
        to: '/patterns',
        label: 'Patterns',
        icon: (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="20" x2="18" y2="10" />
                <line x1="12" y1="20" x2="12" y2="4" />
                <line x1="6" y1="20" x2="6" y2="14" />
            </svg>
        ),
    },
]

export default function TabBar({ onAddClick }) {
    const location = useLocation()

    return (
        <nav className="tab-bar">
            {tabs.map(tab => (
                <NavLink
                    key={tab.to}
                    to={tab.to}
                    className={({ isActive }) => `tab-item${isActive ? ' tab-active' : ''}`}
                >
                    {tab.icon}
                    <span>{tab.label}</span>
                </NavLink>
            ))}

            {/* Center Add button */}
            <button className="tab-add-btn" onClick={onAddClick} aria-label="Add task">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <line x1="12" y1="5" x2="12" y2="19" />
                    <line x1="5" y1="12" x2="19" y2="12" />
                </svg>
            </button>

            {rightTabs.map(tab => (
                <NavLink
                    key={tab.to}
                    to={tab.to}
                    className={({ isActive }) => `tab-item${isActive ? ' tab-active' : ''}`}
                >
                    {tab.icon}
                    <span>{tab.label}</span>
                </NavLink>
            ))}
        </nav>
    )
}