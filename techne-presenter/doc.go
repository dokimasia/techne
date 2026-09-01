// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package presenter carries a tool call between a transport and the tool
// that serves it. It holds no domain knowledge: a presenter that knew
// about individual tools would be one piece of code per tool and
// transport pair.
//
// # Dependency position
//
// Imports core and nothing else in this repository. Only the root module
// imports this package. It exists as its own module so that core does
// not carry the MCP SDK.
package presenter
