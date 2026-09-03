---
name: accessibility-design
description: Inclusive design practices for digital products: cognitive accessibility, motor accessibility, and inclusive content. Use when designing for accessibility beyond WCAG compliance.
origin: builtin
---

# Accessibility Design

## When to use
- Designing for inclusive experiences beyond code compliance
- Creating accessible content and interactions
- Considering cognitive, motor, and situational disabilities

## Inclusive design principles
1. **Recognize exclusion**: identify who your design leaves out
2. **Learn from diversity**: edge cases are the mainstream (one-handed phone use, bright sunlight, slow connection)
3. **Solve for one, extend to many**: captions help deaf users AND people in noisy environments
4. **Personalize**: give users control (font size, motion, contrast)

## Cognitive accessibility
- **Plain language**: short sentences, common words, define jargon
- **Clear structure**: headings, bullet points, white space
- **Consistency**: same action = same button everywhere
- **Error prevention**: confirm destructive actions, validate input
- **Reduce cognitive load**: don't show everything at once, progressive disclosure
- **No time pressure**: extendable timeouts, no auto-advancing carousels

## Motor accessibility
- Large touch targets (44pt minimum)
- No precision-required gestures (pinch, drag-to-reorder)
- Alternatives to drag-and-drop (move buttons, up/down arrows)
- Voice control compatibility
- No hover-dependent functionality (touch screens can't hover)

## Visual accessibility
- Color is not the only differentiator (add icons, text labels)
- 4.5:1 contrast for normal text, 3:1 for large
- Support 200% zoom without breaking layout
- Dark mode support
- Motion sensitivity: respect `prefers-reduced-motion`

## Content accessibility
- Alt text for images (describe purpose, not just appearance)
- Captions and transcripts for audio/video
- Descriptive link text (not "click here")
- Language attribute on HTML root
- Reading level: aim for 8th grade when possible

## Testing with real users
- Test with assistive technology users (not just automated tools)
- Test with keyboard only
- Test in bright light and low light
- Test on slow connections
- Test with one hand (mobile)
