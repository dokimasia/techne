// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package presenter serves the tools of a [tool.Registry] to a client of the Model Context
// Protocol.
//
// The presenter reads the name, the description and the schemas of each tool. It passes the
// arguments of a call to [tool.Tool.Execute] and returns the [tool.Result] as the result of
// the call. It never decodes a payload, so a change to the output of a tool does not require
// a change to the presenter.
//
// # Results
//
//   - The structured content of a result is the payload, which the SDK writes into the
//     response without decoding it.
//   - The text block contains the render of the output, which the tool writes for a model,
//     and the payload for an output without a render. The specification recommends the
//     payload for the text block, for a client that does not read structured content. Such a
//     client reads the render.
//   - A failed result is an error result. The specification recommends that a client pass an
//     error result to the model, so that the model can correct its input.
//
// # Faults
//
//   - An error of Execute returns an error result with the text of the error.
//   - A panic in the goroutine of a call returns an error result with the name of the tool,
//     and the stack goes to standard error. The server serves the next call. A panic in a
//     goroutine that a tool starts ends the process.
//   - A payload that is not JSON returns a protocol error, because the SDK does not send a
//     response for a result that it cannot encode.
//
// # Dependency position
//
// Imports the standard library, go.dokimi.dev/techne/tool and
// github.com/modelcontextprotocol/go-sdk/mcp. The root module is the only module that imports
// the presenter, so no other module of techne depends on a transport.
package presenter
