---
name: image-generation
description: Generate and edit images: illustrations, diagrams, logos, icons, and UI mockups. Use when the user asks to create, draw, or generate an image.
origin: builtin
---

# Image Generation

## When to use
- User asks to create, draw, design, render, or generate an image
- Need illustrations, diagrams, logos, icons, or pictures
- Editing or iterating on an existing image

## Prompt structure
Provide a detailed, self-contained description:
1. **Subject**: what is in the image
2. **Style**: art direction (photorealistic, illustration, flat design, etc.)
3. **Composition**: framing, layout, aspect ratio
4. **Colors**: palette or mood
5. **Context**: purpose or placement

## Editing vs generating
- To **change** an image: pass the image path + describe only the change
- To **generate new**: provide only the prompt
- When iterating on a generated image: use the path it reported under "Saved to:"

## Best practices
- Be specific about what you want
- Mention style explicitly (don't assume photorealistic)
- Specify composition (close-up, wide shot, centered, asymmetric)
- Reference colors by name or hex when precision matters
- For logos: specify "vector-style, minimalist, scalable"

## Anti-patterns
- Vague prompts ("make it look good")
- Too many subjects in one image (cluttered)
- Contradictory instructions ("dark and bright")
- Not specifying aspect ratio for UI mockups
