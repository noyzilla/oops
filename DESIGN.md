# Design System & Styling Guidelines

This document governs the visual tokens, UI components, and output styling across the project based on the machine-readable design system standard.

## Aesthetic Direction
- **Tone**: Clean, focused, high-precision engineering aesthetic.
- **Visual Hierarchy**: High-contrast typography, restrained color palette, purposeful whitespace.

## Design Tokens

### Color Palette
- **Background**: `#0a0a0c` (Dark), `#ffffff` (Light)
- **Foreground**: `#f4f4f6` (Dark), `#111113` (Light)
- **Primary Brand**: `#3b82f6` (Accent Blue)
- **Muted Surface**: `#18181b` (Dark border/card), `#f4f4f5` (Light surface)
- **Semantic Success**: `#10b981`
- **Semantic Warning**: `#f59e0b`
- **Semantic Danger**: `#ef4444`

### Typography Scale
- **Display**: System sans-serif / Inter, 32px, font-weight 600
- **Heading**: System sans-serif / Inter, 20px, font-weight 600
- **Body**: System sans-serif / Inter, 14px, font-weight 400, line-height 1.5
- **Monospace**: JetBrains Mono / Fira Code, 13px, font-weight 400

### Spacing & Layout
- **Base Unit**: 4px grid system (`4px`, `8px`, `12px`, `16px`, `24px`, `32px`)
- **Border Radius**: `4px` (Inputs, Badges), `8px` (Cards, Modals)

## Headless & CLI Output Guidelines
For CLI tools, background workers, or backend services without graphical interfaces, this document governs terminal output formatting:
- **Success Messages**: Printed in green or neutral white with descriptive context; no decorative icons.
- **Errors & Warnings**: Printed to stderr with clear causation, remediation hints, and non-zero exit codes.
- **Structured Output**: Support `--json` output flag for automated machine consumption.

## Deep-Dive Design Specifications
In accordance with the Mirror Index Pattern and documentation taxonomy in [docs/README.md](docs/README.md), detailed design system specifications are extracted on demand into `docs/design/`:
- `docs/design/components.md` - Component library tokens, button variants, and state definitions.
- `docs/design/forms.md` - Form input styling, label positioning, and error message typography.
- `docs/design/tables.md` - Table layouts, row hover effects, and pagination controls.
- `docs/design/accessibility.md` - WCAG color contrast standards, focus rings, and keyboard navigation.
