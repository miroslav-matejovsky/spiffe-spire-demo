// Package step provides an interactive step runner for SPIFFE/SPIRE demo scenarios.
//
// Each scenario is a sequence of Steps. A Step has:
//   - Name: a short heading displayed in the step banner
//   - Explain: educational text shown BEFORE the action runs, describing what
//     will happen and why, referencing config files, and calling out concepts
//   - Action: a function that performs the actual work (start container, etc.)
//   - Observe: educational text shown AFTER the action succeeds, explaining
//     what SPIRE did, what to inspect, and suggesting verification commands
//
// The Runner supports two modes:
//   - Auto mode (default): prints Explain, executes Action, prints Observe,
//     continues immediately. Used for fast startup with educational output.
//   - Step mode (--step flag): pauses after Explain so the developer can read
//     the explanation before the action runs, then pauses again after Observe
//     so the developer can run suggested commands and inspect state.
//
// Step mode is the key educational feature. Developers learning SPIFFE/SPIRE
// can take their time at each phase, verify what happened, and build a mental
// model of the system before moving to the next step.
//
// The legacy Run(name, explain, action) method is kept for backward
// compatibility. It delegates to RunStep with an empty Observe field.
package step
