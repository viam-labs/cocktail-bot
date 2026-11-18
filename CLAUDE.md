# CLAUDE.md

## Working in this repo

- Default to no code comments; add terse one-liners only when the WHY (not the WHAT) is non-obvious from the code.
- Do NOT bandaid over upstream bugs or platform gaps in any dependency — raise them immediately with a severity (critical / high / medium / low).

## Platform shortcomings

Known gaps this module cannot fix on its own. Do not paper over them — reference this section when you hit them and, if new ones surface, add them here.

- **`motion.Move` ignores per-pose `vel_degs_per_sec` / `acc_degs_per_sec2`** (medium). The pose-switcher's `set_position_by_name` goes through `motion.Move`, which has no first-class vel/acc fields and whose builtin planner ignores arbitrary `Extra` keys. The bartender's own flow dodges this by calling `armplanning.PlanMotion` + `arm.MoveThroughJointPositions` with explicit `arm.MoveOptions`, so configured per-pose speeds only apply there. Anything that drives a pose through `motion.Move` (app.viam.com's set-pose button, Stream Deck, direct `set_position_by_name` DoCommand) runs at the arm's default speed. Fix would require routing the pose-switcher around `motion.Move` to match the bartender's path.
