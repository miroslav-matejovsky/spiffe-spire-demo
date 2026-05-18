// Package step provides an interactive step runner for SPIFFE/SPIRE demo scenarios.
//
// Each scenario is a sequence of named steps. A step has:
//   - A short name displayed as a heading
//   - An educational explanation of what is happening and why
//   - An action function that performs the actual work
//
// The Runner supports two modes:
//   - Auto mode (default): prints explanation, executes action, continues immediately
//   - Step mode (--step flag): pauses after the explanation, waits for user to press
//     Enter before executing the action. This lets learners read and understand each
//     phase before it runs.
//
// Step mode is the key educational feature. Users running demos for learning can
// take their time understanding SPIFFE concepts at each phase.
package step
