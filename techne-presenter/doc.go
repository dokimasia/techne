// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package presenter carries a tool call between a client and the tool
// that serves it.
//
// # No domain knowledge
//
// It reads a tool's name, description and schemas, and calls
// [tool.Tool.Execute]. It never parses a payload: [tool.Result] says
// whether a caller should read the answer as a failure, so a change to
// what a tool returns does not reach this package.
//
// # A refusal is a result
//
// A language nothing serves, or a request that was declined, comes back
// as a tool execution error rather than a protocol error. The
// specification says clients should hand execution errors to the model
// so it can correct itself, and "Python has no rename planner" is
// something a model can act on.
//
// # Structured and readable
//
// An answer travels as structured content against the tool's declared
// output schema, with the same JSON in a text block, which is what the
// specification asks of a tool returning structured content.
//
// # Dependency position
//
// Imports core/tool and the MCP SDK. It exists as its own module so that
// core carries no transport, and nothing but the root module imports it.
package presenter
