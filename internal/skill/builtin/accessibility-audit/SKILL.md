---
name: accessibility-audit
description: Web accessibility audit following WCAG 2.2 guidelines: keyboard navigation, screen reader support, color contrast, and ARIA. Use when improving a11y compliance.
origin: builtin
---

# Accessibility Audit

## When to use
- Auditing a web application for WCAG 2.2 compliance
- Improving keyboard navigation
- Adding screen reader support
- Fixing color contrast issues

## WCAG 2.2 principles (POUR)
1. **Perceivable**: content must be presentable in ways users can perceive
2. **Operable**: interface must be operable (keyboard, no seizure-inducing content)
3. **Understandable**: content and operation must be understandable
4. **Robust**: content must work with assistive technologies

## Common issues to check
- Missing `alt` text on images
- Insufficient color contrast (4.5:1 for normal text, 3:1 for large)
- No keyboard focus indicators
- Missing `label` elements for form inputs
- Non-semantic HTML (div soup instead of button/nav/main)
- Missing ARIA landmarks
- No skip-to-content link
- Autoplay without controls
- Time limits without warning/extension

## Keyboard navigation
- All interactive elements must be reachable via Tab
- Visible focus indicator (don't remove `:focus` outline without replacement)
- Logical tab order (DOM order, not visual order)
- Enter/Space to activate, Esc to close modals
- Skip links for keyboard users

## Screen reader support
- Use semantic HTML: `<button>`, `<nav>`, `<main>`, `<header>`, `<footer>`
- ARIA only when semantic HTML is insufficient
- `aria-label` for icon-only buttons
- `aria-expanded`, `aria-hidden` for dynamic content
- `role="alert"` for time-sensitive messages
- Don't overuse ARIA — it can make things worse

## Testing tools
- axe DevTools (automated scanning)
- Lighthouse Accessibility audit
- NVDA/VoiceOver (screen reader testing)
- Keyboard-only navigation test
- Color contrast analyzers
