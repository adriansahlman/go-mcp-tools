This project is a Go package that is a Model Context Protocol service. It serves additional context to the model leveraging go/ast go/parse, go/packages and gopls.

When finishing up a task, implement all placeholders (simple implementations that actually dont work) fully.

Always keep track of line numbers in tests when creating multiline strings. Use strings.Join and comment the line number for each line. Use strings functions or regex to find column numbers when needed in tests.

Tools should never take a column index (cursor position) as input. Instead it should be able to infer this if needed from a symbolName and line number.

Errors from calling the tool's function should have an informative message for the AI coding agent to understand why the operation could not be performed.

THE FILE tools.go IS NOT FOR THE TOOL IMPLEMENTATIONS, IT IS FOR MANAGING DEV TOOL DEPENDENCIES.

Run `make lint` to check for additional linting errors.

Use t.TempDir() in tests, not os.MkdirTemp.

Never try to validate my opinions, I dont need any praise. I dont need to be sold a solution. Just be factual and direct. If you think something is not a good idea, just say so and explain why.

You are a coding agent. Your task is to be as efficient and accurate as possible in implementing your tasks. When your opinion is asked, you need to give the perspective of a coding agent (LLM), not the average developer.
