// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package mailservice

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestEmbeddedTemplates checks that omitting the template path loads the very same
// templates from the binary as pointing it at the template source directory.
func TestEmbeddedTemplates(t *testing.T) {
	log := zaptest.NewLogger(t)

	embedded, err := New(log, nil, "", TenantConfig{}, WhiteLabelConfig{}, nil)
	require.NoError(t, err)

	fromDisk, err := New(log, nil, "../../web/satellite/static/emails", TenantConfig{}, WhiteLabelConfig{}, nil)
	require.NoError(t, err)

	htmlEmbedded, htmlDisk := map[string]string{}, map[string]string{}
	for _, tmpl := range embedded.html.Templates() {
		htmlEmbedded[tmpl.Name()] = tmpl.Tree.Root.String()
	}
	for _, tmpl := range fromDisk.html.Templates() {
		htmlDisk[tmpl.Name()] = tmpl.Tree.Root.String()
	}
	require.NotEmpty(t, htmlEmbedded)
	require.Equal(t, names(htmlDisk), names(htmlEmbedded))
	require.Equal(t, htmlDisk, htmlEmbedded)

	textEmbedded, textDisk := map[string]string{}, map[string]string{}
	for _, tmpl := range embedded.text.Templates() {
		textEmbedded[tmpl.Name()] = tmpl.Tree.Root.String()
	}
	for _, tmpl := range fromDisk.text.Templates() {
		textDisk[tmpl.Name()] = tmpl.Tree.Root.String()
	}
	require.NotEmpty(t, textEmbedded)
	require.Equal(t, names(textDisk), names(textEmbedded))
	require.Equal(t, textDisk, textEmbedded)
}

func names(templates map[string]string) []string {
	var result []string
	for name := range templates {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
