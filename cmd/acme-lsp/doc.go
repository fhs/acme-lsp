/*
The program acme-lsp is a client for the acme text editor that
acts as a proxy for a set of Language Server Protocol servers.

A Language Server implements the Language Server Protocol
(see https://langserver.org/), which provides language features
like auto complete, go to definition, find all references, etc.
Acme-lsp depends on one or more language servers already being
installed in the system.  See this page of a list of language servers:
https://microsoft.github.io/language-server-protocol/implementors/servers/.

Acme-lsp executes or connects to a set of LSP servers specified using the
-server or -dial flags. It then listens for messages sent by the L command
to unix domain socket located at $NAMESPACE/acme-lsp.rpc. The messages
direct acme-lsp to run commands on the LSP servers and apply/show the
results in acme. The communication protocol used here is jsonrpc2 (same
as LSP) but it's an implementation detail that is subject to change.

Acme-lsp watches for files created (New), loaded (Get), saved (Put), or
deleted (Del) in acme, and tells the LSP server about these changes. The
LSP server in turn responds by sending diagnostics information (compiler
errors, lint errors, etc.) which are shown in a "/LSP/Diagnostics" window.
Also, when Put is executed in an acme window, acme-lsp organizes import
paths in the window and formats it.

	Usage: acme-lsp [flags]

  -debug
    	turn on debugging prints
  -dial value
    	language server address for filename match (e.g. '\.go$:localhost:4389')
  -server value
    	language server command for filename match (e.g. '\.go$:gopls')
  -workspaces string
    	colon-separated list of initial workspace directories
*/
package main
