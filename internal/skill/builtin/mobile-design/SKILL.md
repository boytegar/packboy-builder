---
name: mobile-design
description: Mobile app design for iOS and Android: screen layouts, navigation patterns, touch interactions, and platform conventions. Use when designing mobile app screens or flows.
origin: builtin
---

# Mobile Design

## When to use
- Designing iOS, Android, or cross-platform mobile app screens
- Creating app screen concepts and user flows
- Designing mobile UI mockups

## Design principles
- **Thumb zone**: primary actions in the bottom third of the screen
- **44pt minimum touch target** (iOS HIG) / 48dp (Material)
- **Safe areas**: respect notches, home indicators, status bars
- **Platform conventions**: iOS uses tab bars/navigation controllers; Android uses bottom nav/app bar
- **Readable text**: 16pt minimum body text, 14pt minimum captions

## Screen types
- **Onboarding**: 3-4 slides, value proposition, skip button
- **Login/Auth**: minimal fields, social auth options, biometric prompt
- **Home/Dashboard**: summary cards, quick actions, navigation
- **List/Feed**: infinite scroll, pull-to-refresh, swipe actions
- **Detail**: hero image, content, action bar
- **Settings**: grouped lists, toggles, section headers
- **Modal/Sheet**: bottom sheet on mobile, not full-screen modal

## Navigation patterns
- **Stack (push/pop)**: drill-down navigation, back button
- **Tab bar**: switch between main sections
- **Modal**: temporary overlay, dismiss to return
- **Drawer**: hamburger menu (avoid for primary navigation)

## Platform specifics
- **iOS**: SF Symbols, system fonts (San Francisco), 8pt grid, rounded corners
- **Android**: Material Icons, Roboto, 8dp grid, elevation/shadows
- **Cross-platform**: design for both, don't copy web patterns

## Dark mode
- Design both light and dark variants
- Use semantic colors (background, surface, text) not hardcoded values
- Test contrast in both modes
- Don't use pure black (#000) for backgrounds — use dark gray

## Anti-patterns
- Web-style dropdowns on mobile (use sheets/pickers instead)
- Text below 12pt
- Touch targets below 44pt
- Horizontal scrolling
- More than 5 tabs in bottom navigation
