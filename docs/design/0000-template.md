---
title: [Component or Design Topic Name]
status: draft
tags: [design, component, ui, template]
synapses: ["DESIGN.md", "docs/specs/0000-template.md"]
---

# Design Spec: [Component Name]

- **Status**: Active | Experimental | Deprecated
- **Parent Reference**: [DESIGN.md](../../DESIGN.md)
- **Target Platform**: Web | Mobile | Desktop | CLI
- **Associated Specs**: [docs/specs/0000-template.md](../specs/0000-template.md)

## Overview & Usage Intent
- **When to Use**: Situations and domain contexts where this component or pattern is recommended.
- **When NOT to Use**: Contexts where this component is prohibited, along with recommended alternatives.

## Visual Anatomy & Design Tokens
Underlying design tokens and styling attributes (hardcoded hex colors and ad-hoc units are prohibited):
- **Color Tokens**: Background, border, foreground, text, and state tint tokens.
- **Typography Tokens**: Font family, scale, weight, and line height tokens.
- **Spacing & Elevation**: Padding, margin, gap, border radius, and shadow tokens.

## Variants & Component States
- **Variants**: Available styles (such as Primary, Secondary, Outline, Ghost, Danger).
- **Interactive States**:
  - `Default`: Rest state under normal conditions.
  - `Hover`: Pointer hover state.
  - `Active / Pressed`: Pointer down / pressed state.
  - `Focus-Visible`: Explicit keyboard focus indicator for accessibility.
  - `Disabled`: Inactive state with reduced opacity and blocked event propagation.
  - `Loading / Busy`: Asynchronous execution state with animated skeleton or progress spinner.

## Accessibility & Keyboard Navigation (A11y)
- **Semantic HTML & ARIA**: Standard semantic elements, ARIA roles, and accessibility attributes.
- **Keyboard Controls**: Navigation keys (`Tab`, `Enter`, `Space`, `Escape`, `Arrow` keys).
- **Color Contrast**: Contrast ratios complying with WCAG 2.1 AA (minimum 4.5:1 for body text).

## Responsive & Layout Behavior
- **Breakpoints**: Adaptive behavior across Mobile, Tablet, and Desktop viewports.
- **Container Constraints**: Fluid sizing behavior, minimum/maximum widths, and alignment rules.

## Anti-Patterns & Misuse
Explicit rules prohibiting common styling and structural mistakes to prevent UI drift:
- Prohibited token overrides or inconsistent colors.
- Prohibited nested interactive elements.
