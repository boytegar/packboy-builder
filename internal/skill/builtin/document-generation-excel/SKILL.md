---
name: document-generation-excel
description: Generate polished Excel spreadsheets: reports, budgets, data exports. Use when the user asks for an Excel file or spreadsheet.
origin: builtin
---

# Excel Generation

## When to use
- User requests an Excel file, spreadsheet, or .xlsx deliverable
- Need structured tabular data with formatting
- Generating reports, budgets, or data exports

## Spreadsheet structure
- **Sheet naming**: descriptive, short (e.g., "Summary", "Q3 Data")
- **Headers**: bold, frozen top row, auto-filter
- **Data types**: numbers right-aligned, dates in ISO format, text left-aligned
- **Formulas**: use for calculations, not hardcoded values
- **Conditional formatting**: highlight outliers, trends
- **Charts**: embed when visual adds value

## Formatting rules
- Column widths: auto-fit or manually set for readability
- Number formats: #,##0 for thousands, 0.00 for decimals, $#,##0.00 for currency
- Date format: YYYY-MM-DD (ISO) or locale-appropriate
- Cell padding: comfortable, not cramped
- Borders: light, only where needed (tables, not every cell)
- Header row: bold + background fill

## Data organization
- One table per sheet (don't mix unrelated data)
- Header row at top, data below, no blank rows in the middle
- Named ranges for referenced data
- Pivot tables for summary views
