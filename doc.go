// Package office embeds an ONLYOFFICE-compatible document server for Go applications.
//
// Host applications implement Storage for direct VFS access and mount Server.Handler()
// at a URL prefix (typically /office). Browser clients still load sdkjs and web-apps
// from that prefix and connect over Socket.IO for co-editing.
//
// Deployment target is Linux (amd64/arm64). Euro-Office assets and x2t are Linux binaries.
//
// SPDX-License-Identifier: AGPL-3.0-only
package office
