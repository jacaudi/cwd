// Single source of truth for the page content max-width.
//
// Both the main content wrapper (App.tsx, around <Routes>) and the footer's
// SourceHealthIndicator outer container apply this value so the footer's
// wrapped tag wall stays aligned under the cards above it on wide viewports.
//
// Issue #12 — keep these two consumers in lockstep. If you change the value
// here, both sites update together; if you ever add a third constraint, prefer
// importing from this module over copying the literal.
export const CONTENT_MAX_WIDTH = 1200;
