# AGENTS.md

- Never commit changes - Never!
- Talk like a Caveman - Short sentences, simple words, no fluff, avoid "the", "a", "an", "this", "that", etc.
- Be concise in output but thorough in reasoning.
- No sycophantic openers or closing fluff.
- Think before acting. Read existing files before writing code.
- Prefer editing over rewriting whole files.
- Do not re-read files you have already read unless the file may have changed.
- Keep solutions simple and direct.
- When unclear, describe the problem and ask for clarification, or write to the local text file and stop.
- Before completing a task, run `task all` to ensure all tests pass and code is formatted. `task all` must pass before work is considered complete!
- After completing a task, update the `.todo` file with any remaining work or future improvements. Be specific and clear in instructions for future work.

## Business Purpose

- Always read `doc.go` files in each package to understand the domain concepts and design decisions before modifying or adding code.
- If a `doc.go` file is missing, create one with comprehensive documentation to explain the domain concepts and design decisions. This is crucial for maintainability and onboarding new developers.
- Always update `doc.go` files in each package to explain any changes to the domain concepts and design decisions. This is crucial for maintainability and onboarding new developers.

## Environment and Tooling

- Use PowerShell only for any shell scripting or terminal commands.
- Use `gopls` to find definitions and references in go code.
- Use `go doc` to understand Go code and libraries.
- Use `require` assertions from the `testify` library for unit tests.

## Coding Style

- Idiomatic go code is required.
- Write clear and concise Go docs for all exported functions, types, and packages.
- Write docs for internal packages and unexported functions as well, but use a less formal style.
- Always prefer composition over inheritance.
- Use KISS principle - simple, direct solutions are preferred over complex ones.
- Use YAGNI principle - do not implement features until they are actually needed.
- Code duplication is acceptable - do not abstract code until you have at least 3 instances of it.
- Avoid premature optimization. Write clear code first, optimize later if needed.

## Output

- Return code first. Explanation after, only if non-obvious.
- No boilerplate unless explicitly requested.
- No em dashes, smart quotes, or decorative Unicode symbols.
- Plain hyphens and straight quotes only.
- Natural language characters (accented letters, CJK, etc.) are fine when the content requires them.
- Code output must be copy-paste safe.
- Pipeline calls compound. Every token saved per call multiplies across runs.
- No explanatory text in agent output unless a human will read it.
- Return the minimum viable output that satisfies the task spec.
- When implementation is not completed or stubs are used, document missing pieces in the `.todo` file with clear instructions in the root of the project.
