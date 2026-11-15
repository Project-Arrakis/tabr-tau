# Saved map filters: mock-up and inventory (refs #679)

- `filters-inventory.md`: every group and control of the reference map's filters section, by its on-screen label, mapped onto what the save supplies, with the gaps in both directions, the count, and the decisions for the operator (each with a recommended default).
- `filters.html`: a static mock-up of the Saved map with those filters. Inline CSS and a short inline script, example data only (marked on the page), no external request; it works from disk at phone width (375 px) and on a desktop, in light and dark. A switch at the top hides the data-source marks. Without the script the folds and switches still work; the group switches, Show all, Hide all, Find and Saved filters need it.

Nothing here changes the editor's code or reads a save.
